package main

import (
	"math"
	"math/rand"
	"os"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/model3d/model3d"
	"github.com/unixpickle/model3d/render3d"
)

// RandomScene creates a random collection of objects and
// fills out a renderer to render them.
func RandomScene(models, images []string) (render3d.Object, *render3d.RecursiveRayTracer) {
	layout := RandomSceneLayout()
	numObjects := rand.Intn(10) + 1
	numLights := rand.Intn(5) + 1

	var objects render3d.JoinedObject
	var focusPoints []render3d.FocusPoint
	var focusProbs []float64

	for _, wall := range layout.CreateBackdrop() {
		objects = append(objects, RandomizeMaterial(wall, images))
	}

	for i := 0; i < numObjects; i++ {
		path := models[rand.Intn(len(models))]
		r, err := os.Open(path)
		essentials.Must(err)
		defer r.Close()
		tris, err := model3d.ReadOFF(r)
		essentials.Must(err)
		mesh := model3d.NewMeshTriangles(tris)
		rotation := model3d.NewMatrix3Rotation(model3d.NewCoord3DRandUnit(),
			rand.Float64()*math.Pi*2)
		mesh = mesh.MapCoords(rotation.MulColumn)
		mesh = layout.PlaceMesh(mesh)
		objects = append(objects, RandomizeMaterial(mesh, images))
	}

	for i := 0; i < numLights; i++ {
		light := layout.CreateLight()
		min, max := light.Min(), light.Max()
		objects = append(objects, light)
		focusPoints = append(focusPoints, &render3d.SphereFocusPoint{
			Center: min.Mid(max),
			Radius: min.Dist(max) / 2,
		})
		focusProbs = append(focusProbs, 0.3/float64(numLights))
	}

	origin, target := layout.CameraInfo()
	fov := (rand.Float64()*0.5 + 0.5) * math.Pi / 3.0
	return objects, &render3d.RecursiveRayTracer{
		Camera:          render3d.NewCameraAt(origin, target, fov),
		FocusPoints:     focusPoints,
		FocusPointProbs: focusProbs,
	}
}

// RandomSceneLayout samples a SceneLayout from some
// distribution.
func RandomSceneLayout() SceneLayout {
	return RoomLayout{
		Width: rand.Float64()*2.0 + 0.5,
		Depth: rand.Float64()*20.0 + 5.0,
	}
}

type SceneLayout interface {
	// CameraInfo determines where the scene would like to
	// setup the camera for rendering.
	CameraInfo() (position, target model3d.Coord3D)

	// CreateLight creates a randomized light object that
	// makes sense in this kind of scene.
	CreateLight() render3d.Object

	// CreateBackdrop creates meshes which act as walls of
	// the scene.
	CreateBackdrop() []*model3d.Mesh

	// PlaceMesh translates and scales the mesh so that it
	// fits within the scene.
	PlaceMesh(m *model3d.Mesh) *model3d.Mesh
}

// RoomLayout is a simple scene in a room with lights on
// the walls and ceiling.
type RoomLayout struct {
	Width float64
	Depth float64
}

func (r RoomLayout) CameraInfo() (position, target model3d.Coord3D) {
	return model3d.Coord3D{Z: 0.5, Y: -r.Depth/2 + 1e-5}, model3d.Coord3D{Z: 0.5, Y: r.Depth / 2}
}

func (r RoomLayout) CreateLight() render3d.Object {
	var center model3d.Coord3D
	if rand.Intn(2) == 0 {
		// Place light on ceiling.
		center = model3d.Coord3D{
			X: (rand.Float64() - 0.5) * r.Width,
			Y: (rand.Float64() - 0.5) * r.Depth,
			Z: 1.0,
		}
	} else {
		// Place light on side wall.
		x := r.Width / 2
		if rand.Intn(2) == 0 {
			x = -x
		}
		center = model3d.Coord3D{
			X: x,
			Y: (rand.Float64() - 0.5) * r.Depth,
			Z: rand.Float64() * 0.9,
		}
	}

	var shape model3d.Collider
	if rand.Intn(2) == 0 {
		shape = &model3d.Sphere{Center: center, Radius: rand.Float64()*0.2 + 0.05}
	} else {
		size := uniformRandom().Scale(0.1).Add(model3d.Coord3D{X: 0.05, Y: 0.05, Z: 0.05})
		shape = &model3d.Rect{
			MinVal: center.Sub(size),
			MaxVal: center.Add(size),
		}
	}

	return &render3d.ColliderObject{
		Collider: shape,
		Material: &render3d.LambertMaterial{
			EmissionColor: render3d.NewColor((rand.Float64() + 0.1) * 10),
		},
	}
}

func (r RoomLayout) CreateBackdrop() []*model3d.Mesh {
	min := model3d.Coord3D{X: -r.Width / 2, Y: -r.Depth / 2}
	max := model3d.Coord3D{X: r.Width / 2, Y: r.Depth / 2, Z: 1}
	mesh := model3d.NewMeshRect(min, max)

	var walls []*model3d.Mesh
	mesh.Iterate(func(t *model3d.Triangle) {
		var neighbor *model3d.Triangle
		for _, n := range mesh.Neighbors(t) {
			if n.Normal().Dot(t.Normal()) > 0.99 {
				neighbor = n
				break
			}
		}
		mesh.Remove(neighbor)
		mesh.Remove(t)
		walls = append(walls, model3d.NewMeshTriangles([]*model3d.Triangle{t, neighbor}))
	})

	return walls
}

func (r RoomLayout) PlaceMesh(m *model3d.Mesh) *model3d.Mesh {
	placeMin := model3d.Coord3D{X: -r.Width / 2}
	placeMax := model3d.Coord3D{X: r.Width / 2, Y: r.Depth / 2, Z: 1}
	return placeInBounds(placeMin, placeMax, m)
}

func placeInBounds(placeMin, placeMax model3d.Coord3D, m *model3d.Mesh) *model3d.Mesh {
	min, max := m.Min(), m.Max()
	diff := max.Sub(min)
	pDiff := placeMax.Sub(placeMin)
	maxScale := math.Min(pDiff.X/diff.X, math.Min(pDiff.Y/diff.Y, pDiff.Z/diff.Z))
	scale := (rand.Float64()*0.9 + 0.1) * maxScale
	m = m.Scale(scale)

	min, max = m.Min(), m.Max()
	translateMin := placeMin.Sub(min)
	translateMax := placeMax.Sub(max)
	translate := uniformRandom().Mul(translateMax.Sub(translateMin))
	return m.MapCoords(translate.Add)
}

func uniformRandom() model3d.Coord3D {
	return model3d.Coord3D{X: rand.Float64(), Y: rand.Float64(), Z: rand.Float64()}
}
