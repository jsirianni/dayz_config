package kamyshowdown

import (
	"encoding/xml"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type Events struct {
	XMLName xml.Name `xml:"events"`
	Text    string   `xml:",chardata"`
	Event   struct {
		Text           string `xml:",chardata"`
		Name           string `xml:"name,attr"`
		Nominal        string `xml:"nominal"`
		Min            string `xml:"min"`
		Max            string `xml:"max"`
		Lifetime       string `xml:"lifetime"`
		Restock        string `xml:"restock"`
		Saferadius     string `xml:"saferadius"`
		Distanceradius string `xml:"distanceradius"`
		Cleanupradius  string `xml:"cleanupradius"`
		Flags          struct {
			Text          string `xml:",chardata"`
			Deletable     string `xml:"deletable,attr"`
			InitRandom    string `xml:"init_random,attr"`
			RemoveDamaged string `xml:"remove_damaged,attr"`
		} `xml:"flags"`
		Position string `xml:"position"`
		Limit    string `xml:"limit"`
		Active   string `xml:"active"`
		Children struct {
			Text  string `xml:",chardata"`
			Child struct {
				Text    string `xml:",chardata"`
				Lootmax string `xml:"lootmax,attr"`
				Lootmin string `xml:"lootmin,attr"`
				Max     string `xml:"max,attr"`
				Min     string `xml:"min,attr"`
				Type    string `xml:"type,attr"`
			} `xml:"child"`
		} `xml:"children"`
	} `xml:"event"`
}

func TestEventsXML(t *testing.T) {
	xmlFile, err := os.Open("../mpmissions/dayzOffline.chernarusplus/db/events.xml")
	if err != nil {
		require.NoError(t, err)
	}
	defer xmlFile.Close()

	b, err := ioutil.ReadAll(xmlFile)
	require.NoError(t, err)

	g := Events{}
	err = xml.Unmarshal(b, &g)
	require.NoError(t, err)
}
