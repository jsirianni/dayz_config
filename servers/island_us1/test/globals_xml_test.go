package islandus1

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type Globals struct {
	XMLName xml.Name `xml:"variables"`
	Text    string   `xml:",chardata"`
	Var     []struct {
		Text  string `xml:",chardata"`
		Name  string `xml:"name,attr"`
		Type  string `xml:"type,attr"`
		Value string `xml:"value,attr"`
	} `xml:"var"`
}

func TestGlobalsXML(t *testing.T) {
	xmlFile, err := os.Open("../mpmissions/dayzOffline.chernarusplus/db/globals.xml")
	if err != nil {
		require.NoError(t, err)
	}
	defer xmlFile.Close()

	b, err := ioutil.ReadAll(xmlFile)
	require.NoError(t, err)

	g := Globals{}
	err = xml.Unmarshal(b, &g)
	require.NoError(t, err)

	for _, line := range g.Var {
		if len(line.Name) == 0 {
			require.NotEmpty(t, line.Name, "expected name to not be empty")
			return
		}

		// All values should convert to int without error
		_, err := strconv.Atoi(line.Value)
		require.NoError(t, err, fmt.Sprintf("expected %s to convert to int without error", line.Name))

		// Right now all "types" are "0" so enforce that for now
		require.Equal(t, "0", line.Type, "expected variable type to be string 0")

		switch name := line.Name; name {
		case "AnimalMaxCount":
			require.Equal(t, "200", line.Value, fmt.Sprintf("expected %s to be 200", name))
		case "CleanupLifetimeDeadPlayer":
			require.Equal(t, "3600", line.Value, fmt.Sprintf("expected %s to be 3600", name))
		case "CleanupLifetimeRuined":
			require.Equal(t, "330", line.Value, fmt.Sprintf("expected %s to be 330", name))
		case "TimeLogin":
			require.Equal(t, "15", line.Value, fmt.Sprintf("expected %s to be 15", name))
		case "TimeLogout":
			require.Equal(t, "15", line.Value, fmt.Sprintf("expected %s to be 15", name))
		case "ZombieMaxCount":
			require.Equal(t, "1000", line.Value, fmt.Sprintf("expected %s to be 1000", name))
		}
	}
}
