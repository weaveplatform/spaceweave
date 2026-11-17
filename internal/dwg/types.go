package dwg

// Fixed object types (ODA specification 20.3).
const (
	typText          = 0x01
	typAttrib        = 0x02
	typAttdef        = 0x03
	typBlock         = 0x04
	typEndblk        = 0x05
	typSeqend        = 0x06
	typInsert        = 0x07
	typMinsert       = 0x08
	typVertex2D      = 0x0A
	typVertex3D      = 0x0B
	typVertexMesh    = 0x0C
	typVertexPface   = 0x0D
	typVertexFace    = 0x0E
	typPolyline2D    = 0x0F
	typPolyline3D    = 0x10
	typArc           = 0x11
	typCircle        = 0x12
	typLine          = 0x13
	typDimOrdinate   = 0x14
	typDimDiameter   = 0x1A
	typPoint         = 0x1B
	typPolylinePface = 0x1D
	typPolylineMesh  = 0x1E
	typSolid         = 0x1F
	typTrace         = 0x20
	typViewport      = 0x22
	typEllipse       = 0x23
	typSpline        = 0x24
	typRay           = 0x28
	typXline         = 0x29
	typMText         = 0x2C
	typBlockControl  = 0x30
	typBlockHeader   = 0x31
	typLayer         = 0x33
	typLWPolyline    = 0x4D
	typHatch         = 0x4E
	typProxyEntity   = 0x1F2
	// typClassBase is where per-drawing class numbers start.
	typClassBase = 500
)

// entityNames are the fixed entity types by DXF name, for counting what
// isn't drawn. A type missing here isn't an entity.
var entityNames = map[int]string{
	typText: "TEXT", typAttrib: "ATTRIB", typAttdef: "ATTDEF", typBlock: "BLOCK",
	typEndblk: "ENDBLK", typSeqend: "SEQEND", typInsert: "INSERT", typMinsert: "INSERT",
	typVertex2D: "VERTEX", typVertex3D: "VERTEX", typVertexMesh: "VERTEX",
	typVertexPface: "VERTEX", typVertexFace: "VERTEX", typPolyline2D: "POLYLINE",
	typPolyline3D: "POLYLINE", typArc: "ARC", typCircle: "CIRCLE", typLine: "LINE",
	0x14: "DIMENSION", 0x15: "DIMENSION", 0x16: "DIMENSION", 0x17: "DIMENSION",
	0x18: "DIMENSION", 0x19: "DIMENSION", typDimDiameter: "DIMENSION", typPoint: "POINT",
	0x1C: "3DFACE", typPolylinePface: "POLYLINE", typPolylineMesh: "POLYLINE",
	typSolid: "SOLID", typTrace: "TRACE", 0x21: "SHAPE", typViewport: "VIEWPORT",
	typEllipse: "ELLIPSE", typSpline: "SPLINE", 0x25: "REGION", 0x26: "3DSOLID",
	0x27: "BODY", typRay: "RAY", typXline: "XLINE", 0x2B: "OLEFRAME", typMText: "MTEXT",
	0x2D: "LEADER", 0x2E: "TOLERANCE", 0x2F: "MLINE", 0x4A: "OLE2FRAME",
	typLWPolyline: "LWPOLYLINE", typHatch: "HATCH", typProxyEntity: "ACAD_PROXY_ENTITY",
}

// notDrawn are entity types that carry nothing a plan view should show, or
// that are parts of other entities; they aren't reported as left out. It
// matches the DXF reader's list, with DWG's structural entities added.
var notDrawn = map[string]bool{
	"VIEWPORT": true, "POINT": true, "ATTDEF": true, "SEQEND": true, "VERTEX": true,
	"XLINE": true, "RAY": true, "BLOCK": true, "ENDBLK": true,
}
