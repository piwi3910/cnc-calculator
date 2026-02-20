package engine

import (
	"math/rand"
	"sort"

	"github.com/piwi3910/SlabCut/internal/model"
)

// GeneticConfig holds parameters for the genetic algorithm optimizer.
type GeneticConfig struct {
	PopulationSize int
	Generations    int
	MutationRate   float64
	TournamentSize int
	EliteCount     int
}

// DefaultGeneticConfig returns sensible default parameters.
func DefaultGeneticConfig() GeneticConfig {
	return GeneticConfig{
		PopulationSize: 50,
		Generations:    100,
		MutationRate:   0.15,
		TournamentSize: 3,
		EliteCount:     2,
	}
}

// gene represents a single part placement decision in the chromosome.
type gene struct {
	partIndex int  // Index into the expanded parts slice
	rotated   bool // Whether this part should be rotated 90 degrees
}

// chromosome represents a candidate solution: an ordering of parts with rotation flags.
type chromosome struct {
	genes   []gene
	fitness float64
}

// geneticOptimizer implements the genetic algorithm for cut optimization.
type geneticOptimizer struct {
	settings model.CutSettings
	config   GeneticConfig
	parts    []model.Part
	stocks   []model.StockSheet
	rng      *rand.Rand
}

// newGeneticOptimizer creates a new genetic optimizer instance.
func newGeneticOptimizer(settings model.CutSettings, config GeneticConfig, parts []model.Part, stocks []model.StockSheet, seed int64) *geneticOptimizer {
	return &geneticOptimizer{
		settings: settings,
		config:   config,
		parts:    parts,
		stocks:   stocks,
		rng:      rand.New(rand.NewSource(seed)),
	}
}

// optimize runs the genetic algorithm and returns the best result.
func (g *geneticOptimizer) optimize() model.OptimizeResult {
	if len(g.parts) == 0 || len(g.stocks) == 0 {
		return model.OptimizeResult{}
	}

	// Initialize population
	population := g.initPopulation()

	// Evaluate initial population
	for i := range population {
		population[i].fitness = g.evaluate(population[i])
	}

	// Evolution loop
	for gen := 0; gen < g.config.Generations; gen++ {
		// Sort by fitness descending (higher is better)
		sort.Slice(population, func(i, j int) bool {
			return population[i].fitness > population[j].fitness
		})

		newPop := make([]chromosome, 0, g.config.PopulationSize)

		// Elitism: carry over the best individuals unchanged
		eliteCount := g.config.EliteCount
		if eliteCount > len(population) {
			eliteCount = len(population)
		}
		for i := 0; i < eliteCount; i++ {
			newPop = append(newPop, g.copyChromosome(population[i]))
		}

		// Fill rest of population with offspring
		for len(newPop) < g.config.PopulationSize {
			parent1 := g.tournamentSelect(population)
			parent2 := g.tournamentSelect(population)

			child := g.orderCrossover(parent1, parent2)

			g.mutate(&child)

			child.fitness = g.evaluate(child)
			newPop = append(newPop, child)
		}

		population = newPop
	}

	// Find best individual
	sort.Slice(population, func(i, j int) bool {
		return population[i].fitness > population[j].fitness
	})

	return g.decode(population[0])
}

// initPopulation creates the initial random population.
func (g *geneticOptimizer) initPopulation() []chromosome {
	n := len(g.parts)
	population := make([]chromosome, g.config.PopulationSize)

	for i := range population {
		genes := make([]gene, n)
		perm := g.rng.Perm(n)
		for j := 0; j < n; j++ {
			canRotate := g.parts[perm[j]].Grain == model.GrainNone
			genes[j] = gene{
				partIndex: perm[j],
				rotated:   canRotate && g.rng.Float64() < 0.5,
			}
		}
		population[i] = chromosome{genes: genes}
	}

	// Also seed one chromosome with the greedy order (largest area first)
	// to give the GA a good starting point
	if g.config.PopulationSize > 0 {
		greedy := g.createGreedyChromosome()
		population[0] = greedy
	}

	return population
}

// createGreedyChromosome creates a chromosome sorted by area descending (mimics greedy heuristic).
func (g *geneticOptimizer) createGreedyChromosome() chromosome {
	n := len(g.parts)
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		ai := g.parts[indices[i]].Width * g.parts[indices[i]].Height
		aj := g.parts[indices[j]].Width * g.parts[indices[j]].Height
		return ai > aj
	})

	genes := make([]gene, n)
	for i, idx := range indices {
		genes[i] = gene{partIndex: idx, rotated: false}
	}
	return chromosome{genes: genes}
}

// evaluate computes the fitness of a chromosome by decoding it into a packing
// and measuring material efficiency.
func (g *geneticOptimizer) evaluate(c chromosome) float64 {
	result := g.decode(c)

	if len(result.Sheets) == 0 {
		return 0
	}

	var usedArea, totalArea float64
	for _, s := range result.Sheets {
		usedArea += s.UsedArea()
		totalArea += s.TotalArea()
	}

	if totalArea == 0 {
		return 0
	}

	efficiency := usedArea / totalArea

	// Penalize unplaced parts heavily
	unplacedPenalty := float64(len(result.UnplacedParts)) * 0.1
	// Penalize using more sheets
	sheetPenalty := float64(len(result.Sheets)-1) * 0.05

	fitness := efficiency - unplacedPenalty - sheetPenalty
	if fitness < 0 {
		fitness = 0
	}
	return fitness
}

// decode converts a chromosome into an actual packing result using the guillotine packer.
func (g *geneticOptimizer) decode(c chromosome) model.OptimizeResult {
	// Build stock pool
	var stockPool []model.StockSheet
	for _, s := range g.stocks {
		for i := 0; i < s.Quantity; i++ {
			cp := s
			cp.Quantity = 1
			stockPool = append(stockPool, cp)
		}
	}

	result := model.OptimizeResult{}

	// Build the ordered list of parts from chromosome
	type partPlacement struct {
		part    model.Part
		rotated bool
	}
	ordered := make([]partPlacement, len(c.genes))
	for i, gene := range c.genes {
		ordered[i] = partPlacement{
			part:    g.parts[gene.partIndex],
			rotated: gene.rotated,
		}
	}

	remaining := ordered
	opt := &Optimizer{Settings: g.settings}

	for len(remaining) > 0 && len(stockPool) > 0 {
		// Extract just the parts for stock selection
		remainingParts := make([]model.Part, len(remaining))
		for i, r := range remaining {
			remainingParts[i] = r.part
		}

		bestStockIdx := opt.selectBestStock(stockPool, remainingParts)
		if bestStockIdx < 0 {
			break
		}

		stock := stockPool[bestStockIdx]
		stockPool = append(stockPool[:bestStockIdx], stockPool[bestStockIdx+1:]...)

		sheet := model.SheetResult{Stock: stock}
		var unplaced []partPlacement

		// Get stock tab configuration
		tabConfig := stock.Tabs
		if !tabConfig.Enabled {
			tabConfig = g.settings.StockTabs
		}

		freeRects := opt.calculateFreeRects(stock, tabConfig)
		packer := newGuillotinePackerWithRects(freeRects, g.settings.KerfWidth)

		for _, pp := range remaining {
			placed := false

			// Check grain compatibility between part and stock sheet
			canNormal, canRotated := model.CanPlaceWithGrain(pp.part.Grain, stock.Grain)

			if pp.rotated && canRotated {
				// Try rotated first (chromosome says rotate)
				if ok, x, y := packer.insert(pp.part.Height, pp.part.Width); ok {
					sheet.Placements = append(sheet.Placements, model.Placement{
						Part:    pp.part,
						X:       x,
						Y:       y,
						Rotated: true,
					})
					placed = true
				}
				// Fall back to normal orientation
				if !placed && canNormal {
					if ok, x, y := packer.insert(pp.part.Width, pp.part.Height); ok {
						sheet.Placements = append(sheet.Placements, model.Placement{
							Part:    pp.part,
							X:       x,
							Y:       y,
							Rotated: false,
						})
						placed = true
					}
				}
			} else {
				// Try normal orientation first
				if canNormal {
					if ok, x, y := packer.insert(pp.part.Width, pp.part.Height); ok {
						sheet.Placements = append(sheet.Placements, model.Placement{
							Part:    pp.part,
							X:       x,
							Y:       y,
							Rotated: false,
						})
						placed = true
					}
				}
				// Fall back to rotated if grain allows
				if !placed && canRotated {
					if ok, x, y := packer.insert(pp.part.Height, pp.part.Width); ok {
						sheet.Placements = append(sheet.Placements, model.Placement{
							Part:    pp.part,
							X:       x,
							Y:       y,
							Rotated: true,
						})
						placed = true
					}
				}
			}

			if !placed {
				unplaced = append(unplaced, pp)
			}
		}

		if len(sheet.Placements) > 0 {
			result.Sheets = append(result.Sheets, sheet)
		}
		remaining = unplaced
	}

	// Collect unplaced parts
	for _, pp := range remaining {
		result.UnplacedParts = append(result.UnplacedParts, pp.part)
	}

	return result
}

// tournamentSelect picks the best individual from a random tournament.
func (g *geneticOptimizer) tournamentSelect(population []chromosome) chromosome {
	best := population[g.rng.Intn(len(population))]
	for i := 1; i < g.config.TournamentSize; i++ {
		candidate := population[g.rng.Intn(len(population))]
		if candidate.fitness > best.fitness {
			best = candidate
		}
	}
	return g.copyChromosome(best)
}

// orderCrossover implements Order Crossover (OX1) for permutation chromosomes.
// It preserves the relative order of genes from both parents.
func (g *geneticOptimizer) orderCrossover(parent1, parent2 chromosome) chromosome {
	n := len(parent1.genes)
	if n <= 2 {
		return g.copyChromosome(parent1)
	}

	// Select two random crossover points
	point1 := g.rng.Intn(n)
	point2 := g.rng.Intn(n)
	if point1 > point2 {
		point1, point2 = point2, point1
	}

	child := chromosome{genes: make([]gene, n)}

	// Copy segment from parent1
	inSegment := make(map[int]bool)
	for i := point1; i <= point2; i++ {
		child.genes[i] = parent1.genes[i]
		inSegment[parent1.genes[i].partIndex] = true
	}

	// Fill remaining positions with genes from parent2 in order
	childIdx := (point2 + 1) % n
	for _, pg := range parent2.genes {
		if !inSegment[pg.partIndex] {
			child.genes[childIdx] = pg
			childIdx = (childIdx + 1) % n
		}
	}

	return child
}

// mutate applies random mutations to a chromosome.
func (g *geneticOptimizer) mutate(c *chromosome) {
	n := len(c.genes)
	if n < 2 {
		return
	}

	// Swap mutation: swap two random genes' positions
	if g.rng.Float64() < g.config.MutationRate {
		i := g.rng.Intn(n)
		j := g.rng.Intn(n)
		c.genes[i], c.genes[j] = c.genes[j], c.genes[i]
	}

	// Rotation mutation: toggle rotation of a random gene (if grain allows)
	if g.rng.Float64() < g.config.MutationRate {
		i := g.rng.Intn(n)
		part := g.parts[c.genes[i].partIndex]
		if part.Grain == model.GrainNone {
			c.genes[i].rotated = !c.genes[i].rotated
		}
	}

	// Inversion mutation: reverse a small segment (less frequent)
	if g.rng.Float64() < g.config.MutationRate*0.5 {
		i := g.rng.Intn(n)
		j := g.rng.Intn(n)
		if i > j {
			i, j = j, i
		}
		for i < j {
			c.genes[i], c.genes[j] = c.genes[j], c.genes[i]
			i++
			j--
		}
	}
}

// copyChromosome creates a deep copy of a chromosome.
func (g *geneticOptimizer) copyChromosome(c chromosome) chromosome {
	genes := make([]gene, len(c.genes))
	copy(genes, c.genes)
	return chromosome{genes: genes, fitness: c.fitness}
}

// OptimizeGenetic runs the genetic algorithm optimizer.
// It expands parts by quantity, then uses the GA to find an optimal ordering.
func OptimizeGenetic(settings model.CutSettings, parts []model.Part, stocks []model.StockSheet) model.OptimizeResult {
	// Expand parts by quantity
	var expanded []model.Part
	for _, p := range parts {
		for i := 0; i < p.Quantity; i++ {
			cp := p
			cp.Quantity = 1
			expanded = append(expanded, cp)
		}
	}

	if len(expanded) == 0 || len(stocks) == 0 {
		return model.OptimizeResult{}
	}

	config := DefaultGeneticConfig()

	// Scale generations for larger problems
	if len(expanded) > 20 {
		config.Generations = 150
	}
	if len(expanded) > 50 {
		config.Generations = 200
		config.PopulationSize = 80
	}

	ga := newGeneticOptimizer(settings, config, expanded, stocks, 42)
	return ga.optimize()
}
