<p align="center"><img src="logos/slabcut-logo.svg" alt="SlabCut" width="400"></p>

# SlabCut

A cross-platform desktop application for optimizing rectangular cut lists and generating CNC-ready GCode. Built with Go and [Fyne](https://fyne.io/) — produces a single native binary for Windows, macOS, and Linux with no runtime dependencies.

## Features

### Optimization Engine
- **2D Bin Packing** — Maximal rectangles optimization with Best Area Fit heuristic and automatic rotation for no-grain parts
- **Genetic Algorithm** — Alternative optimizer using population-based meta-heuristic for better packing efficiency
- **Grain Direction** — Supports horizontal/vertical grain constraints on both parts and stock sheets with automatic grain matching
- **Saw Kerf & Edge Trim** — Accounts for blade width and stock edge waste
- **Part Rotation** — Automatically rotates parts for better fit (respects grain)
- **Multiple Stock Sizes** — Use different stock sheet sizes in one run with smart selection (trial-packing heuristic)
- **Multi-Material Optimization** — Assign material types (e.g., Plywood, MDF) to parts and stock sheets; optimizer groups by material so parts are only placed on matching stocks
- **Interior Cutout Nesting** — Automatically nests smaller parts inside the waste holes of larger parts with cutouts, maximizing material utilization
- **Fixture / Clamp Zones** — Define rectangular exclusion zones for clamps and fixtures; optimizer avoids placing parts in these areas
- **Dust Shoe Collision Detection** — Automatically checks for collisions between the dust shoe and clamp/fixture zones after optimization; warns before generating GCode
- **Multi-Objective Optimization** — Weight priorities for minimize waste, minimize sheets, minimize cut length, and minimize job time; genetic algorithm uses weighted multi-objective fitness scoring
- **Non-Rectangular Nesting** — Outline parts can be rotated at multiple angles (configurable: 2/4/8+ rotations) to find tighter bounding-box fits; includes polygon overlap detection, area calculation, and point-in-polygon testing

### CNC & GCode
- **GCode Export** — Full CNC toolpath generation with:
  - Multi-pass depth stepping
  - Configurable feed/plunge rates and spindle speed
  - Holding tabs to prevent part movement
  - Tool radius compensation (outside cut)
  - Safe Z retract between operations
  - Lead-in/lead-out arcs for smoother entry and exit
  - Toolpath ordering optimization (nearest-neighbor) to minimize rapid travel
  - Ramped and helical plunge entry strategies for reduced tool stress
  - Dogbone and T-bone corner overcuts for square interior corners
  - Onion skinning to leave thin material layer and prevent part movement
  - Structural cut ordering (center-out) to maintain workpiece rigidity during machining
- **GCode Preview** — Visual toolpath simulation with color-coded rapid/feed/plunge moves
- **Toolpath Simulation** — Interactive GCode simulation with progress slider, play/pause/stop/step controls, adjustable speed (0.25x-16x), completed vs remaining cut visualization, live tool position indicator, loop playback, and real-time coordinate display (X/Y/Z/Feed/Type)
- **Live Simulation Viewport** — Dedicated GCode Preview tab with instant toolpath visualization
- **Post-Processor Profiles** — Built-in profiles for Grbl, Mach3, LinuxCNC + custom user profiles
- **DXF Part Outlines** — GCode follows actual part contours for non-rectangular shapes

### Import & Export
- **CSV Import** — Auto-detect delimiters (comma, semicolon, tab, pipe) and column mapping
- **Excel Import** — Read .xlsx files with auto header detection
- **DXF Import** — Import non-rectangular parts from DXF files (LWPOLYLINE, LINE, ARC, CIRCLE)
- **PDF Export** — Multi-page cut diagrams with dimensions, labels, efficiency stats, and summary
- **QR Label Export** — Generate printable QR-coded labels for cut parts with part name, dimensions, and sheet position (Avery 5160 layout)
- **GCode Export** — Per-sheet GCode files with configurable profiles
- **Project Sharing** — Export projects as `.slabshare` files with author, notes, and metadata for team collaboration; import shared projects with preview
- **Project Save/Load** — JSON-based project files (`.cnccalc`)

### User Interface
- **OrcaSlicer-Inspired 2-Tab Layout** — Modern three-pane Layout Editor (quick settings | sheet canvas | parts/stock) and dedicated GCode Preview tab
- **Live Auto-Optimization** — Debounced 500ms auto-optimize on any settings, parts, or stock change for instant visual feedback
- **Visual Layout** — Color-coded sheet diagrams showing part placements and stock tab zones
- **Zoomable Canvas** — Mouse wheel zoom (centered on cursor) and click-and-drag panning on sheet layout views, with zoom in/out buttons and reset
- **Compact Part & Stock Cards** — Inline add bars, edit/delete actions, and accordion-organized panels
- **Quick Settings Panel** — Collapsible accordion sections for Tool, Material, Cutting, and Optimizer parameters
- **Advanced Settings Dialog** — Full CNC settings (optimization weights, GCode profiles, lead-in/out, plunge entry, corner overcuts, onion skinning, toolpath ordering, tabs, clamp zones, dust shoe collision)
- **Undo/Redo** — Full history with Ctrl+Z / Ctrl+Y (Cmd+Z / Cmd+Shift+Z on macOS)
- **Parts Library** — Save and reuse predefined parts organized by category
- **Tool & Stock Inventory** — Manage cutting tools and stock sheet presets
- **Material Pricing** — Track price per sheet in stock inventory; view total material cost in optimization results
- **Offcuts / Remnants Tracking** — Automatically detects usable rectangular remnants after optimization; save offcuts to stock inventory for future projects
- **Purchasing Calculator** — Calculate sheets needed, board feet, and estimated cost with configurable waste factor (Tools menu)
- **Edge Banding Calculator** — Mark which edges need banding per part (T/B/L/R); view per-part and total banding length with waste factor in results
- **What-If Comparison** — Compare multiple optimization scenarios side by side (algorithm, kerf, edge trim) and apply the best result
- **Project Templates** — Save and load reusable project templates (parts, stocks, settings) from the Admin menu
- **Admin Menu** — Application settings, inventory management, data backup/restore
- **Stock Size Presets** — Quick-select dropdown with common panel sizes (Full, Half, Quarter sheet, Euro sizes)

## Prerequisites

- Go 1.22+
- C compiler (GCC/MinGW on Windows, Xcode CLI tools on macOS)
  - Required by Fyne for CGo graphics bindings
- On Linux: `sudo apt install libgl1-mesa-dev xorg-dev` (for OpenGL)

## Build

```bash
# Run directly
make run

# Build for current platform
make build

# Cross-compile
make windows        # produces slabcut.exe
make darwin-arm64   # produces slabcut-darwin-arm64 (Apple Silicon)
make darwin-amd64   # produces slabcut-darwin-amd64 (Intel Mac)
make linux          # produces slabcut-linux
```

### Packaged Builds (recommended for distribution)

Uses [fyne-cross](https://github.com/fyne-io/fyne-cross) for proper `.exe`/`.app` bundles:

```bash
go install github.com/fyne-io/fyne-cross@latest

make package-windows   # Windows .exe with icon
make package-darwin    # macOS .app bundle (universal binary)
```

## Run Tests

```bash
make test
```

## Project Structure

```
SlabCut/
├── cmd/slabcut/
│   └── main.go                 # Entry point
├── internal/
│   ├── model/
│   │   ├── model.go            # Core types (Part, StockSheet, Placement, etc.)
│   │   ├── inventory.go        # Tool/stock inventory types
│   │   ├── library.go          # Parts library types
│   │   ├── template.go         # Project template types
│   │   └── appconfig.go        # Application configuration
│   ├── engine/
│   │   ├── optimizer.go        # Guillotine bin-packing algorithm
│   │   └── genetic.go          # Genetic algorithm optimizer
│   ├── gcode/
│   │   ├── generator.go        # GCode toolpath generation
│   │   └── parser.go           # GCode parser for preview
│   ├── importer/
│   │   ├── importer.go         # CSV/Excel import with auto-detection
│   │   └── dxf.go              # DXF file import
│   ├── export/
│   │   ├── pdf.go              # PDF export of cut diagrams
│   │   └── labels.go           # QR code label generation
│   ├── project/
│   │   ├── project.go          # Save/load project files
│   │   ├── profiles.go         # Custom GCode profile persistence
│   │   ├── inventory.go        # Tool/stock inventory persistence
│   │   ├── library.go          # Parts library persistence
│   │   ├── templates.go        # Project template persistence
│   │   ├── sharing.go          # Project sharing/collaboration
│   │   └── appconfig.go        # App config persistence
│   └── ui/
│       ├── app.go              # Main UI (2-tab layout, menus, dialogs)
│       ├── advanced_settings.go # Advanced CNC settings dialog
│       ├── history.go          # Undo/redo history manager
│       ├── inventory.go        # Inventory management dialogs
│       ├── library.go          # Parts library dialogs
│       ├── admin.go            # Admin menu and settings
│       ├── profile_editor.go   # GCode profile editor
│       └── widgets/
│           ├── sheet_canvas.go    # Visual sheet layout renderer
│           └── gcode_preview.go   # GCode toolpath preview
├── .github/workflows/
│   └── ci.yml                  # CI: build, test, lint, docs check
├── go.mod
├── Makefile
├── CLAUDE.md
└── README.md
```

## Architecture

```mermaid
graph TB
    subgraph "UI Layer (Fyne) — 2-Tab Layout"
        subgraph "Tab 1: Layout Editor (HSplit 3-pane)"
            QS[Quick Settings Panel] --> B[App]
            SC[Sheet Canvas + Zoom] --> B
            PS[Parts & Stock Cards] --> B
        end
        subgraph "Tab 2: GCode Preview"
            GP[GCode Simulation Viewport] --> B
        end
        AD[Advanced Settings Dialog] --> B
        H[Admin Menu] --> B
    end

    subgraph "Live Auto-Optimization"
        B -->|500ms debounce| I[Optimizer]
        I --> J[Guillotine Packer]
        I --> K[Genetic Algorithm]
    end

    subgraph "GCode Generator"
        B --> L[GCode Generator]
        L --> M[Toolpath + Tabs + Lead-in/out]
        L --> N[Post-Processor Profiles]
    end

    subgraph "Import / Export"
        B --> O[CSV/Excel Importer]
        B --> P[DXF Importer]
        B --> Q[PDF Exporter]
        B --> R[GCode Exporter]
    end

    subgraph "Data Layer"
        S[Project Save/Load]
        T[Parts Library]
        U[Tool/Stock Inventory]
        V[App Config]
    end

    style I fill:#e8f5e9
    style K fill:#e8f5e9
    style L fill:#e1f5ff
    style Q fill:#fff4e6
    style T fill:#f3e5f5
    style SC fill:#e1f5ff
    style QS fill:#f3e5f5
```

## Contributing

1. Create a GitHub issue describing the change
2. Create a feature branch: `issue-NUM-description`
3. Implement with tests
4. Update README.md and CLAUDE.md if your changes affect features, architecture, or workflow
5. Create a PR referencing the issue

## License

MIT
