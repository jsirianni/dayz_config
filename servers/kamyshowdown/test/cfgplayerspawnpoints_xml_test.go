package kamyshowdown

import (
	"encoding/xml"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type Playerspawnpoints struct {
	XMLName xml.Name `xml:"playerspawnpoints"`
	Text    string   `xml:",chardata"`
	Fresh   struct {
		Text        string `xml:",chardata"`
		SpawnParams struct {
			Text            string `xml:",chardata"`
			MinDistInfected string `xml:"min_dist_infected"`
			MaxDistInfected string `xml:"max_dist_infected"`
			MinDistPlayer   string `xml:"min_dist_player"`
			MaxDistPlayer   string `xml:"max_dist_player"`
			MinDistStatic   string `xml:"min_dist_static"`
			MaxDistStatic   string `xml:"max_dist_static"`
		} `xml:"spawn_params"`
		GeneratorParams struct {
			Text          string `xml:",chardata"`
			GridDensity   string `xml:"grid_density"`
			GridWidth     string `xml:"grid_width"`
			GridHeight    string `xml:"grid_height"`
			MinDistStatic string `xml:"min_dist_static"`
			MaxDistStatic string `xml:"max_dist_static"`
			MinSteepness  string `xml:"min_steepness"`
			MaxSteepness  string `xml:"max_steepness"`
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
}
