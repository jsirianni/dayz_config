package islandus1

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/fatih/structs"
	"github.com/stretchr/testify/require"
)

type Playerspawnpoints struct {
	XMLName xml.Name `xml:"playerspawnpoints"`
	Text    string   `xml:",chardata"`
	Fresh   struct {
		Text        string `xml:",chardata"`
		SpawnParams struct {
			Text            string  `xml:",chardata"`
			MinDistInfected float64 `xml:"min_dist_infected"`
			MaxDistInfected float64 `xml:"max_dist_infected"`
			MinDistPlayer   float64 `xml:"min_dist_player"`
			MaxDistPlayer   float64 `xml:"max_dist_player"`
			MinDistStatic   float64 `xml:"min_dist_static"`
			MaxDistStatic   float64 `xml:"max_dist_static"`
		} `xml:"spawn_params"`
		GeneratorParams struct {
			Text          string  `xml:",chardata"`
			GridDensity   float64 `xml:"grid_density"`
			GridWidth     float64 `xml:"grid_width"`
			GridHeight    float64 `xml:"grid_height"`
			MinDistStatic float64 `xml:"min_dist_static"`
			MaxDistStatic float64 `xml:"max_dist_static"`
			MinSteepness  float64 `xml:"min_steepness"`
			MaxSteepness  float64 `xml:"max_steepness"`
		} `xml:"generator_params"`
		GeneratorPosbubbles struct {
			Text string `xml:",chardata"`
			Pos  []struct {
				Text string `xml:",chardata"`
				X    string `xml:"x,attr"`
				Z    string `xml:"z,attr"`
			} `xml:"pos"`
		} `xml:"generator_posbubbles"`
	} `xml:"fresh"`
	Hop struct {
		Text                string `xml:",chardata"`
		GeneratorPosbubbles struct {
			Text string `xml:",chardata"`
			Pos  []struct {
				Text string `xml:",chardata"`
				X    string `xml:"x,attr"`
				Z    string `xml:"z,attr"`
			} `xml:"pos"`
		} `xml:"generator_posbubbles"`
	} `xml:"hop"`
	Travel struct {
		Text                string `xml:",chardata"`
		GeneratorPosbubbles struct {
			Text string `xml:",chardata"`
			Pos  []struct {
				Text string `xml:",chardata"`
				X    string `xml:"x,attr"`
				Z    string `xml:"z,attr"`
			} `xml:"pos"`
		} `xml:"generator_posbubbles"`
	} `xml:"travel"`
}

func TestCFGPlayerSpawnPointsXML(t *testing.T) {
	xmlFile, err := os.Open("../mpmissions/dayzOffline.chernarusplus/cfgplayerspawnpoints.xml")
	if err != nil {
		require.NoError(t, err)
	}
	defer xmlFile.Close()

	b, err := ioutil.ReadAll(xmlFile)
	require.NoError(t, err)

	g := Playerspawnpoints{}
	err = xml.Unmarshal(b, &g)
	require.NoError(t, err)

	// Make sure values are greater than 0 and convert to float64
	for k, v := range structs.Map(g.Fresh.SpawnParams) {
		if k == "Text" {
			// Skip the text field
			continue
		}

		require.NotEmpty(t, v, fmt.Sprintf("expected key %s's value to not be empty", k))
		value, ok := v.(string)
		if !ok {
			require.Equal(t, false, ok, fmt.Sprintf("expected value for %s to be a string", k))
			continue
		}
		_, err := strconv.ParseFloat(value, 32)
		require.NoError(t, err, fmt.Sprintf("expected fresh spawn param %s to convert to float64", k))
	}
	for k, v := range structs.Map(g.Fresh.GeneratorParams) {
		if k == "Text" {
			// Skip the text field
			continue
		}

		require.NotEmpty(t, v, fmt.Sprintf("expected key %s's value to not be empty", k))
		value, ok := v.(string)
		if !ok {
			require.Equal(t, false, ok, fmt.Sprintf("expected value for %s to be a string", k))
			continue
		}
		_, err := strconv.ParseFloat(value, 32)
		require.NoError(t, err, fmt.Sprintf("expected fresh generator param %s to convert to float64", k))
	}

	numFreshSpawn := len(g.Fresh.GeneratorPosbubbles.Pos)
	numHopSpawn := len(g.Hop.GeneratorPosbubbles.Pos)
	numTravelSpawn := len(g.Travel.GeneratorPosbubbles.Pos)
	require.Equal(t, numFreshSpawn, numHopSpawn, "expected number of hop spawns to be equal to number of fresh spawns")
	require.Equal(t, numFreshSpawn, numTravelSpawn, "expected number of travel spawns to be equal to number of fresh spawns")
}
