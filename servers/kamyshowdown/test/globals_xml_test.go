package kamyshowdown

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
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

		switch name := line.Name; name {
		case "AnimalMaxCount":
			require.Equal(t, "0", line.Value, fmt.Sprintf("expected %s to be 0", name))
		case "CleanupLifetimeDeadPlayer":
			require.Equal(t, "20", line.Value, fmt.Sprintf("expected %s to be 0", name))
		case "CleanupLifetimeRuined":
			require.Equal(t, "60", line.Value, fmt.Sprintf("expected %s to be 0", name))
		case "TimeLogin":
			require.Equal(t, "3", line.Value, fmt.Sprintf("expected %s to be 0", name))
		case "TimeLogout":
			require.Equal(t, "3", line.Value, fmt.Sprintf("expected %s to be 0", name))
		case "ZombieMaxCount":
			require.Equal(t, "0", line.Value, fmt.Sprintf("expected %s to be 0", name))
		}
	}
}
