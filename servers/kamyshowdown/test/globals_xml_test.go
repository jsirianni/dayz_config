package kamyshowdown

import (
	"encoding/xml"
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
}
