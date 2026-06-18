package config

import (
	"reflect"
	"testing"
)

func TestScenesRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg := Default()
	cfg.Scenes = []Scene{
		{Name: "Movie Night", Description: "Apple TV everywhere", Routes: []int{2, 2, 2, 2}, Slot: 1},
		{Name: "Work", Description: "", Routes: []int{1, 2, 3, 4}, Slot: 0},
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Scenes, cfg.Scenes) {
		t.Fatalf("scenes round-trip mismatch:\n got %+v\nwant %+v", got.Scenes, cfg.Scenes)
	}
}

func TestSceneRoutesMap(t *testing.T) {
	s := Scene{Routes: []int{2, 0, 3, 4}}
	got := s.RoutesMap(4)
	want := map[int]int{1: 2, 3: 3, 4: 4} // output 2 is unset (0) -> omitted
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RoutesMap = %v, want %v", got, want)
	}
}
