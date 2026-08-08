package economy

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dzcli/internal/economyconfig"
)

func TestUpdateTypesXMLModifiesSelectedDuplicateType(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<types>
  <type name="ACOGOptic">
    <nominal>1</nominal>
  </type>
  <type name="ACOGOptic">
    <nominal>2</nominal>
    <lifetime>10</lifetime>
    <restock>0</restock>
    <min>1</min>
    <quantmin>-1</quantmin>
    <quantmax>-1</quantmax>
    <cost>1</cost>
    <flags count_in_cargo="0" count_in_hoarder="0" count_in_map="1" count_in_player="0" crafted="1" deloot="0" />
    <category name="old" />
    <tag name="floor" />
    <usage name="Tenement" />
    <value name="Tier1" />
  </type>
</types>`)

	updated, changed, err := UpdateTypesXML(data, TypeUpdateOptions{
		TypeName:       "ACOGOptic",
		OccurrenceSet:  true,
		Occurrence:     2,
		Rename:         "ACOGOpticFixed",
		Scalars:        map[string]int{"nominal": 3, "lifetime": 20, "restock": 4, "min": 2, "quantmin": 10, "quantmax": 90, "cost": 7},
		RemoveAllFlags: true,
		RemoveFlags:    []string{"crafted"},
		Flags:          map[string]string{"count_in_map": "1", "crafted": "0"},
		Collections: map[string]CollectionUpdate{
			"category": {Set: []string{"tools", "old"}, Remove: []string{"old"}, Add: []string{"weapons"}},
			"tag":      {Clear: true, Add: []string{"fishing"}},
			"usage":    {Remove: []string{"Tenement"}, Add: []string{"Town"}},
			"value":    {Set: []string{"Tier2"}},
		},
	})

	if err != nil {
		t.Fatalf("UpdateTypesXML returned error: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	output := string(updated)
	assertContains(t, output, `<type name="ACOGOptic">`)
	assertContains(t, output, `<type name="ACOGOpticFixed">`)
	assertContains(t, output, `<nominal>3</nominal>`)
	assertContains(t, output, `<flags count_in_map="1" crafted="0" />`)
	assertContains(t, output, `<category name="tools" />`)
	assertContains(t, output, `<category name="weapons" />`)
	assertContains(t, output, `<tag name="fishing" />`)
	assertContains(t, output, `<usage name="Town" />`)
	assertContains(t, output, `<value name="Tier2" />`)
	assertNotContains(t, output, `<usage name="Tenement" />`)
}

func TestUpdateTypesXMLRemovesSingleFlag(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?><types><type name="Flagged"><flags count_in_map="1" crafted="1" /></type></types>`)

	updated, changed, err := UpdateTypesXML(data, TypeUpdateOptions{
		TypeName:    "Flagged",
		RemoveFlags: []string{"crafted"},
		Scalars:     map[string]int{},
		Flags:       map[string]string{},
		Collections: map[string]CollectionUpdate{},
	})

	if err != nil {
		t.Fatalf("UpdateTypesXML returned error: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	assertContains(t, string(updated), `<flags count_in_map="1" />`)
	assertNotContains(t, string(updated), `crafted=`)
}

func TestUpdateTypesXMLRemovesLastFlag(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?><types><type name="Flagged"><flags crafted="1" /></type></types>`)

	updated, changed, err := UpdateTypesXML(data, TypeUpdateOptions{
		TypeName:    "Flagged",
		RemoveFlags: []string{"crafted"},
		Scalars:     map[string]int{},
		Flags:       map[string]string{},
		Collections: map[string]CollectionUpdate{},
	})

	if err != nil {
		t.Fatalf("UpdateTypesXML returned error: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	assertNotContains(t, string(updated), `<flags`)
}

func TestUpdateTypesXMLReturnsOriginalWhenNoSemanticChange(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?><types><type name="Same"><nominal>1</nominal><category name="tools" /></type></types>`)

	updated, changed, err := UpdateTypesXML(data, TypeUpdateOptions{
		TypeName: "Same",
		Scalars:  map[string]int{"nominal": 1},
		Flags:    map[string]string{},
		Collections: map[string]CollectionUpdate{
			"category": {Add: []string{"tools"}},
		},
	})

	if err != nil {
		t.Fatalf("UpdateTypesXML returned error: %v", err)
	}
	if changed {
		t.Fatal("changed = true, want false")
	}
	if !bytes.Equal(updated, data) {
		t.Fatal("updated data changed for semantic no-op")
	}
}

func TestUpdateTypesXMLErrors(t *testing.T) {
	valid := []byte(`<?xml version="1.0" encoding="UTF-8"?><types><type name="One"><nominal>1</nominal></type><type name="One"><nominal>2</nominal></type></types>`)
	tests := []struct {
		name    string
		data    []byte
		options TypeUpdateOptions
		want    string
	}{
		{name: "empty type", data: valid, options: TypeUpdateOptions{}, want: "type name is required"},
		{name: "bad occurrence", data: valid, options: TypeUpdateOptions{TypeName: "One", OccurrenceSet: true}, want: "occurrence"},
		{name: "bad xml", data: []byte(`<types>`), options: TypeUpdateOptions{TypeName: "One"}, want: "XML syntax error"},
		{name: "missing type", data: valid, options: TypeUpdateOptions{TypeName: "Missing"}, want: "not found"},
		{name: "ambiguous type", data: valid, options: TypeUpdateOptions{TypeName: "One"}, want: "use --occurrence"},
		{name: "missing occurrence", data: valid, options: TypeUpdateOptions{TypeName: "One", OccurrenceSet: true, Occurrence: 3}, want: "occurrence 3 not found"},
		{name: "invalid generated xml", data: []byte(`<?xml version="1.0" encoding="UTF-8"?><types><type name="One"><nominal>1</nominal></type></types>`), options: TypeUpdateOptions{TypeName: "One", Scalars: map[string]int{"nominal": -1}}, want: "outside allowed range"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := UpdateTypesXML(test.data, test.options)
			if err == nil {
				t.Fatal("err = nil, want error")
			}
			assertContains(t, err.Error(), test.want)
		})
	}
}

func TestUpdateTypesFileAndWriteFileMutation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.xml")
	writeTestFile(t, path, `<?xml version="1.0" encoding="UTF-8"?><types><type name="Apple"><nominal>1</nominal></type></types>`)

	mutation, err := UpdateTypesFile(path, TypeUpdateOptions{
		TypeName:    "Apple",
		Scalars:     map[string]int{"nominal": 2},
		Flags:       map[string]string{},
		Collections: map[string]CollectionUpdate{},
	})
	if err != nil {
		t.Fatalf("UpdateTypesFile returned error: %v", err)
	}
	if !mutation.Changed {
		t.Fatal("mutation.Changed = false, want true")
	}
	if err := WriteFileMutation(path, mutation); err != nil {
		t.Fatalf("WriteFileMutation returned error: %v", err)
	}
	assertContains(t, readTestFile(t, path), `<nominal>2</nominal>`)

	if err := WriteFileMutation(path, FileMutation{}); err != nil {
		t.Fatalf("no-op WriteFileMutation returned error: %v", err)
	}
	if _, err := UpdateTypesFile(filepath.Join(dir, "missing.xml"), TypeUpdateOptions{}); err == nil {
		t.Fatal("missing file err = nil, want error")
	}
	if _, err := UpdateTypesFile(dir, TypeUpdateOptions{}); err == nil {
		t.Fatal("directory read err = nil, want error")
	}
	if _, err := UpdateTypesFile(path, TypeUpdateOptions{TypeName: "Missing"}); err == nil {
		t.Fatal("UpdateTypesFile mutation err = nil, want error")
	}
	if err := WriteFileMutation(dir, FileMutation{Changed: true, Data: []byte("x"), Mode: 0o600}); err == nil {
		t.Fatal("directory write err = nil, want error")
	}
}

func TestUpdateLimitsXMLAddRemoveAndIdempotent(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<lists>
  <categories>
    <category name="tools" />
  </categories>
  <tags>
    <tag name="floor" />
  </tags>
  <usageflags>
    <usage name="Town" />
  </usageflags>
  <valueflags>
    <value name="Tier1" />
  </valueflags>
</lists>`)

	updated, changed, err := UpdateLimitsXML(data, "tag", "fishing", LimitAdd)
	if err != nil {
		t.Fatalf("UpdateLimitsXML add returned error: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	assertContains(t, string(updated), `<tag name="fishing" />`)

	again, changed, err := UpdateLimitsXML(updated, "tag", "fishing", LimitAdd)
	if err != nil {
		t.Fatalf("UpdateLimitsXML idempotent add returned error: %v", err)
	}
	if changed || !bytes.Equal(again, updated) {
		t.Fatal("idempotent add changed data")
	}

	removed, changed, err := UpdateLimitsXML(updated, "tag", "floor", LimitRemove)
	if err != nil {
		t.Fatalf("UpdateLimitsXML remove returned error: %v", err)
	}
	if !changed {
		t.Fatal("remove changed = false, want true")
	}
	assertNotContains(t, string(removed), `<tag name="floor" />`)

	again, changed, err = UpdateLimitsXML(removed, "tag", "floor", LimitRemove)
	if err != nil {
		t.Fatalf("UpdateLimitsXML idempotent remove returned error: %v", err)
	}
	if changed || !bytes.Equal(again, removed) {
		t.Fatal("idempotent remove changed data")
	}
}

func TestUpdateLimitsXMLErrors(t *testing.T) {
	valid := []byte(`<?xml version="1.0" encoding="UTF-8"?><lists><categories><category name="tools" /></categories></lists>`)
	tests := []struct {
		name   string
		data   []byte
		kind   string
		value  string
		action LimitAction
		want   string
	}{
		{name: "unsupported kind", data: valid, kind: "class", value: "x", action: LimitAdd, want: "unsupported limits kind"},
		{name: "missing name", data: valid, kind: "category", action: LimitAdd, want: "name is required"},
		{name: "unsupported action", data: valid, kind: "category", value: "x", action: "bad", want: "unsupported limits action"},
		{name: "invalid xml", data: []byte(`<lists>`), kind: "category", value: "x", action: LimitAdd, want: "XML syntax error"},
		{name: "missing section", data: valid, kind: "tag", value: "x", action: LimitAdd, want: "section not found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := UpdateLimitsXML(test.data, test.kind, test.value, test.action)
			if err == nil {
				t.Fatal("err = nil, want error")
			}
			assertContains(t, err.Error(), test.want)
		})
	}
}

func TestUpdateLimitsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfglimitsdefinition.xml")
	writeTestFile(t, path, `<?xml version="1.0" encoding="UTF-8"?><lists><categories><category name="tools" /></categories><tags/><usageflags/><valueflags/></lists>`)

	mutation, err := UpdateLimitsFile(path, "category", "weapons", LimitAdd)
	if err != nil {
		t.Fatalf("UpdateLimitsFile returned error: %v", err)
	}
	if !mutation.Changed {
		t.Fatal("mutation.Changed = false, want true")
	}

	if _, err := UpdateLimitsFile(filepath.Join(t.TempDir(), "missing.xml"), "category", "weapons", LimitAdd); err == nil {
		t.Fatal("missing limits file err = nil, want error")
	}
	if _, err := UpdateLimitsFile(path, "missing", "weapons", LimitAdd); err == nil {
		t.Fatal("limits mutation err = nil, want error")
	}
}

func TestUpdateUserLimitGroupXMLModifiesGroupsAndMembers(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<user_lists>
  <usageflags>
    <user name="TownVillage">
      <usage name="Town" />
    </user>
  </usageflags>
  <valueflags>
    <user name="TierGroup">
      <value name="Tier1" />
    </user>
  </valueflags>
</user_lists>`)

	updated, changed, err := UpdateUserLimitGroupXML(data, UserLimitGroupOptions{
		Kind:      "usage",
		GroupName: "Tenement",
		Members:   []string{"Town", "Town", "Village"},
		Action:    UserGroupAdd,
	})
	if err != nil {
		t.Fatalf("group add returned error: %v", err)
	}
	if !changed {
		t.Fatal("group add changed = false, want true")
	}
	assertContains(t, string(updated), `<user name="Tenement">`)
	assertContains(t, string(updated), `<usage name="Village" />`)

	updated, changed, err = UpdateUserLimitGroupXML(updated, UserLimitGroupOptions{
		Kind:      "usage",
		GroupName: "Tenement",
		Members:   []string{"Coast"},
		Action:    UserGroupAdd,
	})
	if err != nil {
		t.Fatalf("existing group add returned error: %v", err)
	}
	if !changed {
		t.Fatal("existing group add changed = false, want true")
	}
	assertContains(t, string(updated), `<usage name="Coast" />`)

	updated, changed, err = UpdateUserLimitGroupXML(updated, UserLimitGroupOptions{Kind: "usage", GroupName: "Tenement", Member: "Military", Action: UserGroupMemberAdd})
	if err != nil {
		t.Fatalf("member add returned error: %v", err)
	}
	if !changed {
		t.Fatal("member add changed = false, want true")
	}
	assertContains(t, string(updated), `<usage name="Military" />`)

	updated, changed, err = UpdateUserLimitGroupXML(updated, UserLimitGroupOptions{Kind: "usage", GroupName: "Tenement", Member: "Coast", Action: UserGroupMemberRemove})
	if err != nil {
		t.Fatalf("member remove returned error: %v", err)
	}
	if !changed {
		t.Fatal("member remove changed = false, want true")
	}
	assertNotContains(t, string(updated), `<usage name="Coast" />`)

	updated, changed, err = UpdateUserLimitGroupXML(updated, UserLimitGroupOptions{Kind: "usage", GroupName: "Tenement", Action: UserGroupRemove})
	if err != nil {
		t.Fatalf("group remove returned error: %v", err)
	}
	if !changed {
		t.Fatal("group remove changed = false, want true")
	}
	assertNotContains(t, string(updated), `<user name="Tenement">`)

	again, changed, err := UpdateUserLimitGroupXML(updated, UserLimitGroupOptions{Kind: "usage", GroupName: "Tenement", Action: UserGroupRemove})
	if err != nil {
		t.Fatalf("idempotent group remove returned error: %v", err)
	}
	if changed || !bytes.Equal(again, updated) {
		t.Fatal("idempotent group remove changed data")
	}
}

func TestUpdateUserLimitGroupXMLNoopsExistingMember(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?><user_lists><usageflags><user name="TownVillage"><usage name="Town" /></user></usageflags><valueflags /></user_lists>`)

	updated, changed, err := UpdateUserLimitGroupXML(data, UserLimitGroupOptions{Kind: "usage", GroupName: "TownVillage", Member: "Town", Action: UserGroupMemberAdd})

	if err != nil {
		t.Fatalf("UpdateUserLimitGroupXML returned error: %v", err)
	}
	if changed {
		t.Fatal("changed = true, want false")
	}
	if !bytes.Equal(updated, data) {
		t.Fatal("data changed for no-op member add")
	}
}

func TestUpdateUserLimitGroupXMLErrors(t *testing.T) {
	valid := []byte(`<?xml version="1.0" encoding="UTF-8"?><user_lists><usageflags><user name="TownVillage"><usage name="Town" /></user></usageflags><valueflags /></user_lists>`)
	tests := []struct {
		name    string
		data    []byte
		options UserLimitGroupOptions
		want    string
	}{
		{name: "unsupported kind", data: valid, options: UserLimitGroupOptions{Kind: "tag", GroupName: "x", Action: UserGroupAdd}, want: "only usage and value"},
		{name: "missing group", data: valid, options: UserLimitGroupOptions{Kind: "usage", Action: UserGroupAdd}, want: "group name is required"},
		{name: "unsupported action", data: valid, options: UserLimitGroupOptions{Kind: "usage", GroupName: "x", Action: "bad"}, want: "unsupported user group action"},
		{name: "invalid xml", data: []byte(`<user_lists>`), options: UserLimitGroupOptions{Kind: "usage", GroupName: "x", Action: UserGroupAdd}, want: "XML syntax error"},
		{name: "missing section", data: []byte(`<?xml version="1.0" encoding="UTF-8"?><user_lists><valueflags /></user_lists>`), options: UserLimitGroupOptions{Kind: "usage", GroupName: "x", Action: UserGroupAdd}, want: "section not found"},
		{name: "unsupported child", data: []byte(`<?xml version="1.0" encoding="UTF-8"?><user_lists><usageflags><user name="Broken"><tag name="x" /></user></usageflags><valueflags /></user_lists>`), options: UserLimitGroupOptions{Kind: "usage", GroupName: "x", Action: UserGroupAdd}, want: "expected <usage>"},
		{name: "missing group member add", data: valid, options: UserLimitGroupOptions{Kind: "usage", GroupName: "Missing", Member: "Town", Action: UserGroupMemberAdd}, want: "not found"},
		{name: "missing group member remove", data: valid, options: UserLimitGroupOptions{Kind: "usage", GroupName: "Missing", Member: "Town", Action: UserGroupMemberRemove}, want: "not found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := UpdateUserLimitGroupXML(test.data, test.options)
			if err == nil {
				t.Fatal("err = nil, want error")
			}
			assertContains(t, err.Error(), test.want)
		})
	}
}

func TestUpdateUserLimitGroupFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfglimitsdefinitionuser.xml")
	writeTestFile(t, path, `<?xml version="1.0" encoding="UTF-8"?><user_lists><usageflags /><valueflags /></user_lists>`)

	mutation, err := UpdateUserLimitGroupFile(path, UserLimitGroupOptions{Kind: "value", GroupName: "TierGroup", Members: []string{"Tier1"}, Action: UserGroupAdd})
	if err != nil {
		t.Fatalf("UpdateUserLimitGroupFile returned error: %v", err)
	}
	if !mutation.Changed {
		t.Fatal("mutation.Changed = false, want true")
	}
	assertContains(t, string(mutation.Data), `<value name="Tier1" />`)

	if _, err := UpdateUserLimitGroupFile(filepath.Join(t.TempDir(), "missing.xml"), UserLimitGroupOptions{Kind: "value", GroupName: "TierGroup", Action: UserGroupAdd}); err == nil {
		t.Fatal("missing user limits file err = nil, want error")
	}
	if _, err := UpdateUserLimitGroupFile(path, UserLimitGroupOptions{Kind: "missing", GroupName: "TierGroup", Action: UserGroupAdd}); err == nil {
		t.Fatal("user limits mutation err = nil, want error")
	}
}

func TestHelperErrorPaths(t *testing.T) {
	if _, err := findStartOffset([]byte("<typename></typename>"), len("<typename>"), "type"); err == nil {
		t.Fatal("findStartOffset err = nil, want error")
	}
	if _, err := findTypeRanges([]byte(`<types><x:type name="Bad"></x:type></types>`), "Bad"); err == nil {
		t.Fatal("findTypeRanges namespace err = nil, want error")
	}
	if _, err := findTypeRanges([]byte(`<`), "Bad"); err == nil {
		t.Fatal("findTypeRanges parser err = nil, want error")
	}
	if _, err := findElementRange([]byte(`<lists><x:tags></x:tags></lists>`), "tags"); err == nil {
		t.Fatal("findElementRange namespace err = nil, want error")
	}
	if _, err := findElementRange([]byte(`<`), "tags"); err == nil {
		t.Fatal("findElementRange parser err = nil, want error")
	}

	decoder := xml.NewDecoder(strings.NewReader(`<type><child>`))
	token, err := decoder.Token()
	if err != nil {
		t.Fatalf("decoder start token: %v", err)
	}
	if _, ok := token.(xml.StartElement); !ok {
		t.Fatal("first token is not start element")
	}
	if _, err := consumeElement(decoder, "type"); err == nil {
		t.Fatal("consumeElement err = nil, want error")
	}
	mismatchedDecoder := xml.NewDecoder(strings.NewReader(`</other>`))
	if _, err := consumeElement(mismatchedDecoder, "type"); err == nil {
		t.Fatal("consumeElement mismatch err = nil, want error")
	}
	wrongEndDecoder := xml.NewDecoder(strings.NewReader(`<other></other>`))
	if _, err := wrongEndDecoder.Token(); err != nil {
		t.Fatalf("wrong end decoder start token: %v", err)
	}
	if _, err := consumeElement(wrongEndDecoder, "type"); err == nil {
		t.Fatal("consumeElement wrong end err = nil, want error")
	}

	if _, err := decodeEditableType([]byte(`<not-type />`)); err == nil {
		t.Fatal("decodeEditableType err = nil, want error")
	}
	if _, err := decodeEditableType([]byte(`text<type name="Text" />`)); err != nil {
		t.Fatalf("decodeEditableType char data returned error: %v", err)
	}
	if _, err := decodeEditableType([]byte(``)); err == nil {
		t.Fatal("decodeEditableType empty err = nil, want error")
	}
	if _, err := decodeEditableType([]byte(`<type name="Bad">`)); err == nil {
		t.Fatal("decodeEditableType child eof err = nil, want error")
	}
	if _, err := decodeEditableType([]byte(`<type name="Bad"><nominal><child>`)); err == nil {
		t.Fatal("decodeEditableType scalar child err = nil, want error")
	}
	if _, err := decodeEditableType([]byte(`<type name="Nested"><nominal><child /></nominal><extra /></type>`)); err != nil {
		t.Fatalf("decodeEditableType nested helper returned error: %v", err)
	}
	if _, err := decodeEditableType([]byte(`<type name="Bad"><flags count_in_map="1">`)); err == nil {
		t.Fatal("decodeEditableType flags err = nil, want error")
	}
	if _, err := decodeEditableType([]byte(`<type name="Bad"><category name="tools">`)); err == nil {
		t.Fatal("decodeEditableType collection err = nil, want error")
	}
	if _, err := decodeEditableType([]byte(`<type name="Bad"><extra>`)); err == nil {
		t.Fatal("decodeEditableType unknown err = nil, want error")
	}
	textDecoder := xml.NewDecoder(strings.NewReader(`<nominal><child>`))
	if token, err := textDecoder.Token(); err != nil {
		t.Fatalf("text decoder start token: %v", err)
	} else if _, ok := token.(xml.StartElement); !ok {
		t.Fatal("text decoder first token is not start element")
	}
	if _, err := decodeTextElement(textDecoder, "nominal"); err == nil {
		t.Fatal("decodeTextElement child err = nil, want error")
	}
	eofTextDecoder := xml.NewDecoder(strings.NewReader(`<nominal>`))
	if _, err := eofTextDecoder.Token(); err != nil {
		t.Fatalf("eof text decoder start token: %v", err)
	}
	if _, err := decodeTextElement(eofTextDecoder, "nominal"); err == nil {
		t.Fatal("decodeTextElement eof err = nil, want error")
	}
	if _, err := decodeFlatListNames([]byte(`<tags><tag name="floor">`), "tag"); err == nil {
		t.Fatal("decodeFlatListNames err = nil, want error")
	}
	if _, err := decodeFlatListNames([]byte(`<tags><`), "tag"); err == nil {
		t.Fatal("decodeFlatListNames parser err = nil, want error")
	}
	if _, err := findTypeRanges([]byte(`<types><type name="Bad">`), "Bad"); err == nil {
		t.Fatal("findTypeRanges err = nil, want error")
	}
	if _, err := findElementRange([]byte(`<lists><tags>`), "tags"); err == nil {
		t.Fatal("findElementRange err = nil, want error")
	}
	if _, err := decodeUserGroups([]byte(`<usageflags><user name="Bad"><tag name="x" /></user></usageflags>`), "usage"); err == nil {
		t.Fatal("decodeUserGroups unknown child err = nil, want error")
	}
	if _, err := decodeUserGroups([]byte(`<usageflags><group name="Other" /></usageflags>`), "usage"); err != nil {
		t.Fatalf("decodeUserGroups unknown group returned error: %v", err)
	}
	if _, err := decodeUserGroups([]byte(`<usageflags><`), "usage"); err == nil {
		t.Fatal("decodeUserGroups parser err = nil, want error")
	}
	if _, err := decodeUserGroups([]byte(`<usageflags><group><child>`), "usage"); err == nil {
		t.Fatal("decodeUserGroups skip err = nil, want error")
	}
	if _, err := decodeUserGroups([]byte(`<usageflags><user name="Bad"><usage name="Town">`), "usage"); err == nil {
		t.Fatal("decodeUserGroups malformed member err = nil, want error")
	}
	if _, err := decodeUserGroups([]byte(`<usageflags><user name="Bad">`), "usage"); err == nil {
		t.Fatal("decodeUserGroups malformed user err = nil, want error")
	}
	if _, _, err := applyUserGroupUpdate(nil, UserLimitGroupOptions{Action: "bad"}); err == nil {
		t.Fatal("applyUserGroupUpdate bad action err = nil, want error")
	}
}

func TestInjectedGuardErrors(t *testing.T) {
	originalParseTypesData := parseTypesData
	originalParseLimitsDefinitionData := parseLimitsDefinitionData
	originalAppendUserLimitsDefinitionData := appendUserLimitsDefinitionData
	originalDecodeEditableType := decodeEditableTypeFunc
	originalDecodeFlatListNames := decodeFlatListNamesFunc
	defer func() {
		parseTypesData = originalParseTypesData
		parseLimitsDefinitionData = originalParseLimitsDefinitionData
		appendUserLimitsDefinitionData = originalAppendUserLimitsDefinitionData
		decodeEditableTypeFunc = originalDecodeEditableType
		decodeFlatListNamesFunc = originalDecodeFlatListNames
	}()

	parseTypesData = func([]byte, string) (economyconfig.TypesFile, error) {
		return economyconfig.TypesFile{}, nil
	}
	if _, _, err := UpdateTypesXML([]byte(`<`), TypeUpdateOptions{TypeName: "Bad"}); err == nil {
		t.Fatal("UpdateTypesXML find ranges err = nil, want error")
	}

	decodeEditableTypeFunc = func([]byte) (editableType, error) {
		return editableType{}, errors.New("decode failed")
	}
	if _, _, err := UpdateTypesXML([]byte(`<?xml version="1.0" encoding="UTF-8"?><types><type name="Bad" /></types>`), TypeUpdateOptions{TypeName: "Bad", Rename: "Other"}); err == nil {
		t.Fatal("UpdateTypesXML decode err = nil, want error")
	}
	decodeEditableTypeFunc = originalDecodeEditableType
	parseTypesData = originalParseTypesData

	parseLimitsCalls := 0
	parseLimitsDefinitionData = func([]byte, string) (economyconfig.LimitsDefinition, error) {
		parseLimitsCalls++
		if parseLimitsCalls == 2 {
			return economyconfig.LimitsDefinition{}, errors.New("post limits validation failed")
		}
		return economyconfig.LimitsDefinition{}, nil
	}
	if _, _, err := UpdateLimitsXML([]byte(`<?xml version="1.0" encoding="UTF-8"?><lists><tags /></lists>`), "tag", "fishing", LimitAdd); err == nil {
		t.Fatal("UpdateLimitsXML post validation err = nil, want error")
	}
	parseLimitsDefinitionData = originalParseLimitsDefinitionData

	decodeFlatListNamesFunc = func([]byte, string) ([]string, error) {
		return nil, errors.New("flat decode failed")
	}
	if _, _, err := UpdateLimitsXML([]byte(`<?xml version="1.0" encoding="UTF-8"?><lists><tags /></lists>`), "tag", "fishing", LimitAdd); err == nil {
		t.Fatal("UpdateLimitsXML flat decode err = nil, want error")
	}
	decodeFlatListNamesFunc = originalDecodeFlatListNames

	appendUserCalls := 0
	appendUserLimitsDefinitionData = func([]byte, string, *economyconfig.LimitsDefinition) error {
		appendUserCalls++
		if appendUserCalls == 2 {
			return errors.New("post user validation failed")
		}
		return nil
	}
	if _, _, err := UpdateUserLimitGroupXML([]byte(`<?xml version="1.0" encoding="UTF-8"?><user_lists><usageflags /></user_lists>`), UserLimitGroupOptions{Kind: "usage", GroupName: "Tenement", Action: UserGroupAdd}); err == nil {
		t.Fatal("UpdateUserLimitGroupXML post validation err = nil, want error")
	}
}

func TestSmallHelpers(t *testing.T) {
	assertEqual(t, detectLineEnding([]byte("a\r\nb")), "\r\n")
	assertEqual(t, detectLineEnding([]byte("a\nb")), "\n")
	assertEqual(t, detectChildIndent([]byte("<type></type>"), "\t"), "\t  ")
	assertEqual(t, detectGrandchildIndent([]byte("<usageflags></usageflags>"), "    "), "      ")
	assertEqual(t, detectEmptyElementEnd([]byte(`<tag name="floor"/>`)), "/>")
	assertEqual(t, leadingWhitespace(" \tvalue"), " \t")
	assertEqual(t, escapeAttribute(`a&b<c>"'`), "a&amp;b&lt;c&gt;&#34;&#39;")
	assertEqual(t, containsString([]string{"a"}, "a"), true)
	assertEqual(t, containsString([]string{"a"}, "b"), false)
	assertEqual(t, equalStrings([]string{"a"}, []string{"a"}), true)
	assertEqual(t, equalStrings([]string{"a"}, []string{"b"}), false)
	assertEqual(t, equalStrings([]string{"a"}, []string{"a", "b"}), false)
	assertEqual(t, findGroupIndex([]userGroup{{Name: "A"}}, "A"), 0)
	assertEqual(t, findGroupIndex([]userGroup{{Name: "A"}}, "B"), -1)
	assertEqual(t, attributeValue(xml.StartElement{Name: xml.Name{Local: "tag"}}, "name"), "")
	assertEqual(t, renderFlagAttributes(map[string]string{"custom_flag": "1"}), `custom_flag="1"`)

	members, changed := addMembers([]string{"a"}, []string{"a"})
	assertEqual(t, changed, false)
	assertEqual(t, strings.Join(members, ","), "a")
}

func TestResolveTypesFileForType(t *testing.T) {
	corePath := fixturePath(t, "mission", "cfgeconomycore.xml")

	resolved, err := ResolveTypesFileForType(corePath, "ModdedItem")

	if err != nil {
		t.Fatalf("ResolveTypesFileForType returned error: %v", err)
	}
	assertContains(t, resolved, filepath.Join("mods", "valid_types.xml"))

	_, err = ResolveTypesFileForType(corePath, "")
	if err == nil {
		t.Fatal("ResolveTypesFileForType empty type err = nil")
	}
	_, err = ResolveTypesFileForType(filepath.Join(t.TempDir(), "missing.xml"), "Apple")
	if err == nil {
		t.Fatal("ResolveTypesFileForType missing core err = nil")
	}
	_, err = ResolveTypesFileForType(corePath, "Missing")
	if err == nil {
		t.Fatal("ResolveTypesFileForType missing type err = nil")
	}

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "cfgeconomycore.xml"), `<?xml version="1.0" encoding="UTF-8"?><economycore><ce folder="mods"><file name="dup.xml" type="types" /></ce></economycore>`)
	writeTestFile(t, filepath.Join(root, "db", "types.xml"), `<?xml version="1.0" encoding="UTF-8"?><types><type name="Dup" /></types>`)
	writeTestFile(t, filepath.Join(root, "mods", "dup.xml"), `<?xml version="1.0" encoding="UTF-8"?><types><type name="Dup" /></types>`)
	_, err = ResolveTypesFileForType(filepath.Join(root, "cfgeconomycore.xml"), "Dup")
	if err == nil {
		t.Fatal("ResolveTypesFileForType duplicate err = nil")
	}

	badRefRoot := t.TempDir()
	writeTestFile(t, filepath.Join(badRefRoot, "cfgeconomycore.xml"), `<?xml version="1.0" encoding="UTF-8"?><economycore><ce folder="mods"><file name="bad.xml" type="types" /></ce></economycore>`)
	writeTestFile(t, filepath.Join(badRefRoot, "db", "types.xml"), `<?xml version="1.0" encoding="UTF-8"?><types></types>`)
	writeTestFile(t, filepath.Join(badRefRoot, "mods", "bad.xml"), `<?xml version="1.0" encoding="UTF-8"?><types>`)
	if _, err := ResolveTypesFileForType(filepath.Join(badRefRoot, "cfgeconomycore.xml"), "Broken"); err == nil {
		t.Fatal("ResolveTypesFileForType bad referenced types err = nil")
	}
}

func TestListLimitNamesFile(t *testing.T) {
	path := fixturePath(t, "mission", "cfglimitsdefinition.xml")
	for _, kind := range []string{"category", "tag", "usage", "value"} {
		names, err := ListLimitNamesFile(path, kind)
		if err != nil {
			t.Fatalf("ListLimitNamesFile(%s) returned error: %v", kind, err)
		}
		if len(names) == 0 {
			t.Fatalf("ListLimitNamesFile(%s) returned no names", kind)
		}
	}
	if _, err := ListLimitNamesFile(path, "bad"); err == nil {
		t.Fatal("ListLimitNamesFile unsupported kind err = nil")
	}
	if _, err := ListLimitNamesFile(filepath.Join(t.TempDir(), "missing.xml"), "tag"); err == nil {
		t.Fatal("ListLimitNamesFile missing file err = nil")
	}
}

func TestListUserLimitGroupsFile(t *testing.T) {
	path := fixturePath(t, "mission", "cfglimitsdefinitionuser.xml")
	groups, err := ListUserLimitGroupsFile(path, "usage")
	if err != nil {
		t.Fatalf("ListUserLimitGroupsFile returned error: %v", err)
	}
	assertEqual(t, groups[0].Name, "CustomUsageGroup")
	assertEqual(t, groups[0].Members[0], "Military")

	groups, err = ListUserLimitGroupsFile(path, "value")
	if err != nil {
		t.Fatalf("ListUserLimitGroupsFile value returned error: %v", err)
	}
	assertEqual(t, groups[0].Name, "CustomValueGroup")

	if _, err := ListUserLimitGroupsFile(path, "tag"); err == nil {
		t.Fatal("ListUserLimitGroupsFile unsupported kind err = nil")
	}
	if _, err := ListUserLimitGroupsFile(filepath.Join(t.TempDir(), "missing.xml"), "usage"); err == nil {
		t.Fatal("ListUserLimitGroupsFile missing file err = nil")
	}
	if _, err := ListUserLimitGroupsFile(writeTempXML(t, `<user_lists>`), "usage"); err == nil {
		t.Fatal("ListUserLimitGroupsFile malformed err = nil")
	}
	if _, err := ListUserLimitGroupsFile(writeTempXML(t, `<?xml version="1.0" encoding="UTF-8"?><user_lists><valueflags /></user_lists>`), "usage"); err == nil {
		t.Fatal("ListUserLimitGroupsFile missing section err = nil")
	}
	if _, err := ListUserLimitGroupsFile(writeTempXML(t, `<?xml version="1.0" encoding="UTF-8"?><user_lists><usageflags><user name="A"><tag name="x" /></user></usageflags><valueflags /></user_lists>`), "usage"); err == nil {
		t.Fatal("ListUserLimitGroupsFile bad member err = nil")
	}

	sortedPath := writeTempXML(t, `<?xml version="1.0" encoding="UTF-8"?><user_lists><usageflags><user name="B"><usage name="Village" /></user><user name="A"><usage name="Town" /></user></usageflags><valueflags /></user_lists>`)
	groups, err = ListUserLimitGroupsFile(sortedPath, "usage")
	if err != nil {
		t.Fatalf("ListUserLimitGroupsFile sorted returned error: %v", err)
	}
	assertEqual(t, groups[0].Name, "A")
	assertEqual(t, groups[1].Name, "B")
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("make test dir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test file %s: %v", path, err)
	}
	return string(data)
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%q does not contain %q", haystack, needle)
	}
}

func assertNotContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("%q contains %q", haystack, needle)
	}
}

func assertEqual[T comparable](t *testing.T, got T, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	args := append([]string{filepath.Dir(file), "..", "..", "testdata"}, parts...)
	return filepath.Clean(filepath.Join(args...))
}

func writeTempXML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file.xml")
	writeTestFile(t, path, content)
	return path
}
