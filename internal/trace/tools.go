package trace

import (
	"fmt"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/weaveplatform/spaceweave/internal/plan"
)

// Tool names.
const (
	toolView     = "view_image"
	toolScan     = "scan_line"
	toolWalls    = "detect_walls"
	toolSubmit   = "submit_trace"
	toolOverlay  = "view_overlay"
	toolGetTrace = "get_trace"
	toolFinish   = "finish"
)

func obj(props map[string]any, required ...string) anthropic.BetaToolInputSchemaParam {
	return anthropic.BetaToolInputSchemaParam{
		Properties:  props,
		Required:    required,
		ExtraFields: map[string]any{"additionalProperties": false},
	}
}

var (
	integer = map[string]any{"type": "integer"}
	number  = map[string]any{"type": "number"}
	str     = map[string]any{"type": "string"}
	point   = map[string]any{"type": "array", "items": number, "minItems": 2, "maxItems": 2}
	polygon = map[string]any{"type": "array", "items": point, "minItems": 3}
)

func enum(
	values ...string,
) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

func region(desc string) map[string]any {
	return map[string]any{
		"x0": integer, "y0": integer, "x1": integer, "y1": integer,
		"grid": map[string]any{"type": "integer", "description": desc},
	}
}

// traceSchema describes plan.Trace for submit_trace. Kept loose where Check
// gives a better message than a schema rejection would (furniture fields
// depend on the type, so they're all optional here).
func traceSchema() map[string]any {
	opening := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": number, "b": number,
			"kind":  enum("door", "double-door", "window", "opening"),
			"swing": enum("N", "S", "E", "W"),
			"hinge": enum("a", "b"),
			"sill":  number, "head": number,
		},
		"required": []string{"a", "b", "kind"},
	}
	wall := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": str, "kind": enum("exterior", "interior", "glass"), "from": point, "to": point,
			"thickness": number, "ext": number,
			"openings": map[string]any{"type": "array", "items": opening},
		},
		"required": []string{"id", "kind", "from", "to"},
	}
	room := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": str, "name": str, "zone": enum(plan.Zones...), "floor": enum(plan.Floors...),
			"polygon": polygon, "label": point,
		},
		"required": []string{"id", "name", "zone", "floor", "polygon"},
	}
	patch := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":      str,
			"name":    str,
			"floor":   enum(plan.Floors...),
			"polygon": polygon,
		},
		"required": []string{"id", "floor", "polygon"},
	}
	types := make([]string, 0, len(plan.Types))
	for k := range plan.Types {
		types = append(types, k)
	}
	slices.Sort(types)
	item := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": enum(types...), "x": number, "y": number,
			"w": number, "d": number, "h": number, "r": number, "rot": number, "facing": number,
			"size": number, "label": str, "mat": enum(plan.Materials...),
			"z0": number, "z1": number, "steps": integer, "rise": number,
		},
		"required": []string{"type", "x", "y"},
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": str,
			"calibration": map[string]any{
				"type":       "object",
				"properties": map[string]any{"pxPerMetre": point, "evidence": str},
				"required":   []string{"pxPerMetre", "evidence"},
			},
			"thickness": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"exterior": number,
					"interior": number,
					"glass":    number,
				},
				"required": []string{"exterior", "interior", "glass"},
			},
			"outline":      map[string]any{"type": "array", "items": point},
			"entrance":     point,
			"walls":        map[string]any{"type": "array", "items": wall},
			"rooms":        map[string]any{"type": "array", "items": room},
			"floorPatches": map[string]any{"type": "array", "items": patch},
			"furniture":    map[string]any{"type": "array", "items": item},
		},
		"required": []string{
			"title",
			"calibration",
			"thickness",
			"entrance",
			"walls",
			"rooms",
			"furniture",
		},
	}
}

// furnitureGuide lists each furniture type's fields for the system prompt,
// generated from plan.Types so the prompt can't drift from what Check accepts.
func furnitureGuide() string {
	types := make([]string, 0, len(plan.Types))
	for k := range plan.Types {
		types = append(types, k)
	}
	slices.Sort(types)
	var b strings.Builder
	b.WriteString("# Furniture types\n\nEach item has type, x, y (pixels). Sizes are metres.\n\n")
	for _, name := range types {
		spec := plan.Types[name]
		fmt.Fprintf(&b, "- `%s`", name)
		if len(spec.Required) > 0 {
			fmt.Fprintf(&b, ": needs %s", strings.Join(spec.Required, ", "))
		}
		if len(spec.Defaults) > 0 {
			keys := make([]string, 0, len(spec.Defaults))
			for k := range spec.Defaults {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			parts := make([]string, 0, len(keys))
			for _, k := range keys {
				parts = append(parts, fmt.Sprintf("%s=%g", k, spec.Defaults[k]))
			}
			fmt.Fprintf(&b, "; defaults %s", strings.Join(parts, ", "))
		}
		switch spec.Oriented {
		case "rot":
			b.WriteString("; `rot` turns it")
		case "facing":
			b.WriteString("; `facing` is the way a person using it looks")
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(
		&b,
		"\n`block` is anything else; `mat` is one of %s. Stairs rise along `facing`; `w` is across the flight, `d` along it.\n",
		strings.Join(plan.Materials, ", "),
	)
	return b.String()
}

func tools() []anthropic.BetaToolUnionParam {
	defs := []anthropic.BetaToolParam{
		{
			Name: toolView,
			Description: anthropic.String(
				"Shows a region of the source image, enlarged up to 4× (or reduced to fit), " +
					"with source-pixel rulers on the top and left edges. Omit the region for the whole image. " +
					"grid draws a light grid every that many source pixels.",
			),
			InputSchema: obj(region("grid spacing in source pixels; 0 for none")),
		},
		{
			Name: toolScan,
			Description: anthropic.String(
				"Lists the runs of dark (ink) pixels along one row or column of the source image, " +
					"between from and to. Use it to find exact wall faces: a wall crossed at right angles is one run " +
					"whose start and end are its two faces.",
			),
			InputSchema: obj(map[string]any{
				"axis": enum("row", "col"), "at": integer, "from": integer, "to": integer,
			}, "axis", "at", "from", "to"),
		},
		{
			Name: toolWalls,
			Description: anthropic.String(
				"Proposes axis-aligned walls: bands of ink at least minLength pixels long " +
					"whose thickness is between minThickness and maxThickness pixels. Returns centrelines and thicknesses, " +
					"longest first. It over-reports (furniture edges, bold text) and misses slanted walls: " +
					"confirm candidates against the picture. Omit the region for the whole image.",
			),
			InputSchema: obj(map[string]any{
				"x0": integer, "y0": integer, "x1": integer, "y1": integer,
				"minThickness": integer, "maxThickness": integer, "minLength": integer,
			}, "minThickness", "maxThickness", "minLength"),
		},
		{
			Name: toolSubmit,
			Description: anthropic.String(
				"Submits the complete trace, replacing any earlier submission. Replies with " +
					"structural problems, the site validator's errors, and the trace drawn over the image. " +
					"Send the whole trace every time.",
			),
			InputSchema:         obj(map[string]any{"trace": traceSchema()}, "trace"),
			EagerInputStreaming: anthropic.Bool(true),
		},
		{
			Name: toolOverlay,
			Description: anthropic.String(
				"Shows a region of your latest submission drawn over the source image, enlarged, with rulers.",
			),
			InputSchema: obj(region("grid spacing in source pixels; 0 for none")),
		},
		{
			Name:        toolGetTrace,
			Description: anthropic.String("Returns your latest submitted trace as JSON."),
			InputSchema: obj(map[string]any{}),
		},
		{
			Name: toolFinish,
			Description: anthropic.String(
				"Ends the job. Only accepted when your latest submission has no problems. " +
					"confidence: 0–1, how faithful the trace is. summary: one line describing the floor. " +
					"caveats: what you approximated, guessed or left out.",
			),
			InputSchema: obj(map[string]any{
				"confidence": number,
				"summary":    str,
				"caveats":    map[string]any{"type": "array", "items": str},
			}, "confidence", "summary", "caveats"),
		},
	}
	out := make([]anthropic.BetaToolUnionParam, len(defs))
	for i := range defs {
		out[i] = anthropic.BetaToolUnionParam{OfTool: &defs[i]}
	}
	return out
}
