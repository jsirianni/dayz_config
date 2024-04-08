package kamyshowdown

import (
	"encoding/xml"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type TerritoryType struct {
	XMLName   xml.Name `xml:"territory-type"`
	Text      string   `xml:",chardata"`
	Territory struct {
		Text  string `xml:",chardata"`
		Color string `xml:"color,attr"`
		Zone  struct {
			Text string `xml:",chardata"`
			Name string `xml:"name,attr"`
			Smin string `xml:"smin,attr"`
			Smax string `xml:"smax,attr"`
			Dmin string `xml:"dmin,attr"`
			Dmax string `xml:"dmax,attr"`
			X    string `xml:"x,attr"`
			Z    string `xml:"z,attr"`
			R    string `xml:"r,attr"`
		} `xml:"zone"`
	} `xml:"territory"`
} 

func TestTerritoryTypeXML(t *testing.T) {
	xmlFile, err := os.Open("../mpmissions/dayzOffline.chernarusplus/env/bear_territories.xml")
	if err != nil {
		require.NoError(t, err)
	}
	defer xmlFile.Close()

	b, err := ioutil.ReadAll(xmlFile)
	require.NoError(t, err)

	g := TerritoryType{}
	err = xml.Unmarshal(b, &g)
	require.NoError(t, err)
}
