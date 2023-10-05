// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/multierror"
)

type results struct {
	Expected []json.RawMessage
}

type packageRootTestFinder struct {
	packageRootPath string
}

func (p packageRootTestFinder) FindPackageRoot() (string, bool, error) {
	return p.packageRootPath, true, nil
}

func TestValidate_NoWildcardFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("../../test/packages/parallel/aws/data_stream/elb_logs", WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	f := readTestResults(t, "../../test/packages/parallel/aws/data_stream/elb_logs/_dev/test/pipeline/test-alb.log-expected.json")
	for _, e := range f.Expected {
		errs := validator.ValidateDocumentBody(e)
		require.Empty(t, errs)
	}
}

func TestValidate_WithWildcardFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("../../test/packages/parallel/aws/data_stream/sns", WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/parallel/aws/data_stream/sns/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithFlattenedFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/flattened.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithNumericKeywordFields(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithNumericKeywordFields([]string{"foo.code"}),
		WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/numeric.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithEnabledImportAllECSSchema(t *testing.T) {
	finder := packageRootTestFinder{"../../test/packages/other/imported_mappings_tests"}

	validator, err := createValidatorForDirectoryAndPackageRoot("../../test/packages/other/imported_mappings_tests/data_stream/first",
		finder,
		WithSpecVersion("2.3.0"),
		WithEnabledImportAllECSSChema(true))
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/other/imported_mappings_tests/data_stream/first/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithDisabledImportAllECSSchema(t *testing.T) {
	finder := packageRootTestFinder{"../../test/packages/other/imported_mappings_tests"}

	validator, err := createValidatorForDirectoryAndPackageRoot("../../test/packages/other/imported_mappings_tests/data_stream/first",
		finder,
		WithSpecVersion("2.3.0"),
		WithEnabledImportAllECSSChema(false))
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/other/imported_mappings_tests/data_stream/first/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Len(t, errs, 4)

	errorMessages := []string{}
	for _, err := range errs {
		errorMessages = append(errorMessages, err.Error())
	}
	sort.Strings(errorMessages)
	require.Contains(t, errorMessages[0], `field "destination.geo.location.lat" is undefined`)
}

func TestValidate_constantKeyword(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata", WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/constant-keyword-invalid.json")
	errs := validator.ValidateDocumentBody(e)
	require.NotEmpty(t, errs)

	e = readSampleEvent(t, "testdata/constant-keyword-valid.json")
	errs = validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_ipAddress(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata", WithEnabledAllowedIPCheck(), WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "testdata/ip-address-forbidden.json")
	errs := validator.ValidateDocumentBody(e)
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), `the IP "98.76.54.32" is not one of the allowed test IPs`)

	e = readSampleEvent(t, "testdata/ip-address-allowed.json")
	errs = validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_WithSpecVersion(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata", WithSpecVersion("2.0.0"), WithDisabledDependencyManagement())
	require.NoError(t, err)

	e := readSampleEvent(t, "testdata/invalid-array-normalization.json")
	errs := validator.ValidateDocumentBody(e)
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), `field "container.image.tag" is not normalized as expected`)

	e = readSampleEvent(t, "testdata/valid-array-normalization.json")
	errs = validator.ValidateDocumentBody(e)
	require.Empty(t, errs)

	// Check now that this validation was only enabled for 2.0.0.
	validator, err = CreateValidatorForDirectory("testdata", WithSpecVersion("1.99.99"), WithDisabledDependencyManagement())
	require.NoError(t, err)

	e = readSampleEvent(t, "testdata/invalid-array-normalization.json")
	errs = validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidate_ExpectedEventType(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata", WithSpecVersion("2.0.0"), WithDisabledDependencyManagement())
	require.NoError(t, err)
	require.NotNil(t, validator)

	cases := []struct {
		title string
		doc   common.MapStr
		valid bool
	}{
		{
			title: "valid event type",
			doc: common.MapStr{
				"event.category": "authentication",
				"event.type":     []interface{}{"info"},
			},
			valid: true,
		},
		{
			title: "no event type",
			doc: common.MapStr{
				"event.category": "authentication",
			},
			valid: true,
		},
		{
			title: "multiple valid event type",
			doc: common.MapStr{
				"event.category": "network",
				"event.type":     []interface{}{"protocol", "connection", "end"},
			},
			valid: true,
		},
		{
			title: "multiple categories",
			doc: common.MapStr{
				"event.category": []interface{}{"iam", "configuration"},
				"event.type":     []interface{}{"group", "change"},
			},
			valid: true,
		},
		{
			title: "unexpected event type",
			doc: common.MapStr{
				"event.category": "authentication",
				"event.type":     []interface{}{"access"},
			},
			valid: false,
		},
		{
			title: "multiple categories, no match",
			doc: common.MapStr{
				"event.category": []interface{}{"iam", "configuration"},
				"event.type":     []interface{}{"denied", "end"},
			},
			valid: false,
		},
		{
			title: "multiple categories, some types don't match",
			doc: common.MapStr{
				"event.category": []interface{}{"iam", "configuration"},
				"event.type":     []interface{}{"denied", "end", "group", "change"},
			},
			valid: false,
		},
		{
			title: "multi-field",
			doc: common.MapStr{
				"process.name":      "elastic-package",
				"process.name.text": "elastic-package",
			},
			valid: true,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			errs := validator.ValidateDocumentMap(c.doc)
			if c.valid {
				assert.Empty(t, errs, "should not have errors")
			} else {
				if assert.Len(t, errs, 1, "should have one error") {
					assert.Contains(t, errs[0].Error(), "is not one of the expected values")
				}
			}
		})
	}
}

func TestValidate_ExpectedDatasets(t *testing.T) {
	validator, err := CreateValidatorForDirectory("testdata",
		WithSpecVersion("2.0.0"),
		WithExpectedDatasets([]string{"apache.status"}),
		WithDisabledDependencyManagement(),
	)
	require.NoError(t, err)
	require.NotNil(t, validator)

	cases := []struct {
		title string
		doc   common.MapStr
		valid bool
	}{
		{
			title: "valid dataset",
			doc: common.MapStr{
				"event.dataset": "apache.status",
			},
			valid: true,
		},
		{
			title: "empty dataset",
			doc: common.MapStr{
				"event.dataset": "",
			},
			valid: false,
		},
		{
			title: "absent dataset",
			doc:   common.MapStr{},
			valid: true,
		},
		{
			title: "wrong dataset",
			doc: common.MapStr{
				"event.dataset": "httpd.status",
			},
			valid: false,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			errs := validator.ValidateDocumentMap(c.doc)
			if c.valid {
				assert.Empty(t, errs)
			} else {
				if assert.Len(t, errs, 1) {
					assert.Contains(t, errs[0].Error(), `field "event.dataset" should have value`)
				}
			}
		})
	}
}

func Test_parseElementValue(t *testing.T) {
	for _, test := range []struct {
		key         string
		value       interface{}
		definition  FieldDefinition
		fail        bool
		assertError func(t *testing.T, err error)
		specVersion semver.Version
	}{
		// Arrays
		{
			key:   "string array to keyword",
			value: []interface{}{"hello", "world"},
			definition: FieldDefinition{
				Type: "keyword",
			},
		},
		{
			key:   "numeric string array to long",
			value: []interface{}{"123", "42"},
			definition: FieldDefinition{
				Type: "long",
			},
			fail: true,
		},
		{
			key:   "mixed numbers and strings in number array",
			value: []interface{}{123, "hi"},
			definition: FieldDefinition{
				Type: "long",
			},
			fail: true,
		},

		// keyword and constant_keyword (string)
		{
			key:   "constant_keyword with pattern",
			value: "some value",
			definition: FieldDefinition{
				Type:    "constant_keyword",
				Pattern: `^[a-z]+\s[a-z]+$`,
			},
		},
		{
			key:   "constant_keyword fails pattern",
			value: "some value",
			definition: FieldDefinition{
				Type:    "constant_keyword",
				Pattern: `[0-9]`,
			},
			fail: true,
		},
		// keyword and constant_keyword (other)
		{
			key:   "bad type for keyword",
			value: map[string]interface{}{},
			definition: FieldDefinition{
				Type: "keyword",
			},
			fail: true,
		},
		// date
		{
			key:   "date",
			value: "2020-11-02T18:01:03Z",
			definition: FieldDefinition{
				Type:    "date",
				Pattern: "^[0-9]{4}(-[0-9]{2}){2}[T ][0-9]{2}(:[0-9]{2}){2}Z$",
			},
		},
		{
			key:   "date as milliseconds",
			value: float64(1420070400001),
			definition: FieldDefinition{
				Type: "date",
			},
		},
		{
			key:   "date as milisecond with pattern",
			value: float64(1420070400001),
			definition: FieldDefinition{
				Type:    "date",
				Pattern: "^[0-9]{4}(-[0-9]{2}){2}[T ][0-9]{2}(:[0-9]{2}){2}Z$",
			},
			fail: true,
		},
		{
			key:   "bad date",
			value: "10 Oct 2020 3:42PM",
			definition: FieldDefinition{
				Type:    "date",
				Pattern: "^[0-9]{4}(-[0-9]{2}){2}[T ][0-9]{2}(:[0-9]{2}){2}Z$",
			},
			fail: true,
		},
		// ip
		{
			key:   "ip",
			value: "127.0.0.1",
			definition: FieldDefinition{
				Type:    "ip",
				Pattern: "^[0-9.]+$",
			},
		},
		{
			key:   "bad ip",
			value: "localhost",
			definition: FieldDefinition{
				Type:    "ip",
				Pattern: "^[0-9.]+$",
			},
			fail: true,
		},
		{
			key:   "ip in allowed list",
			value: "1.128.3.4",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "ipv6 in allowed list",
			value: "2a02:cf40:add:4002:91f2:a9b2:e09a:6fc6",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "unspecified ipv6",
			value: "::",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "unspecified ipv4",
			value: "0.0.0.0",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "ipv4 broadcast address",
			value: "255.255.255.255",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "ipv6 min multicast",
			value: "ff00::",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "ipv6 max multicast",
			value: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "abbreviated ipv6 in allowed list with leading 0",
			value: "2a02:cf40:0add:0::1",
			definition: FieldDefinition{
				Type: "ip",
			},
		},
		{
			key:   "ip not in geoip database",
			value: "8.8.8.8",
			definition: FieldDefinition{
				Type: "ip",
			},
			fail: true,
		},
		// text
		{
			key:   "text",
			value: "some text",
			definition: FieldDefinition{
				Type: "text",
			},
		},
		{
			key:   "text with pattern",
			value: "more text",
			definition: FieldDefinition{
				Type:    "ip",
				Pattern: "[A-Z]",
			},
			fail: true,
		},
		// float
		{
			key:   "float",
			value: 3.1416,
			definition: FieldDefinition{
				Type: "float",
			},
		},
		// long
		{
			key:   "bad long",
			value: "65537",
			definition: FieldDefinition{
				Type: "long",
			},
			fail: true,
		},
		// allowed values
		{
			key:   "allowed values",
			value: "configuration",
			definition: FieldDefinition{
				Type: "keyword",
				AllowedValues: AllowedValues{
					{
						Name: "configuration",
					},
					{
						Name: "network",
					},
				},
			},
		},
		{
			key:   "not allowed value",
			value: "display",
			definition: FieldDefinition{
				Type: "keyword",
				AllowedValues: AllowedValues{
					{
						Name: "configuration",
					},
					{
						Name: "network",
					},
				},
			},
			fail: true,
		},
		{
			key:   "not allowed value in array",
			value: []string{"configuration", "display"},
			definition: FieldDefinition{
				Type: "keyword",
				AllowedValues: AllowedValues{
					{
						Name: "configuration",
					},
					{
						Name: "network",
					},
				},
			},
			fail: true,
		},
		// expected values
		{
			key:   "expected values",
			value: "linux",
			definition: FieldDefinition{
				Type:           "keyword",
				ExpectedValues: []string{"linux", "windows"},
			},
		},
		{
			key:   "not expected values",
			value: "bsd",
			definition: FieldDefinition{
				Type:           "keyword",
				ExpectedValues: []string{"linux", "windows"},
			},
			fail: true,
		},
		// fields shouldn't be stored in groups
		{
			key:   "host",
			value: "42",
			definition: FieldDefinition{
				Type: "group",
			},
			fail: true,
		},
		// arrays of objects can be stored in groups, even if not recommended
		{
			key: "host",
			value: []interface{}{
				map[string]interface{}{
					"id":       "somehost-id",
					"hostname": "somehost",
				},
				map[string]interface{}{
					"id":       "otherhost-id",
					"hostname": "otherhost",
				},
			},
			definition: FieldDefinition{
				Name: "host",
				Type: "group",
				Fields: []FieldDefinition{
					{
						Name: "id",
						Type: "keyword",
					},
					{
						Name: "hostname",
						Type: "keyword",
					},
				},
			},
		},
		// elements in arrays of objects should be validated
		{
			key: "details",
			value: []interface{}{
				map[string]interface{}{
					"id":       "somehost-id",
					"hostname": "somehost",
				},
			},
			definition: FieldDefinition{
				Name: "details",
				Type: "group",
				Fields: []FieldDefinition{
					{
						Name: "id",
						Type: "keyword",
					},
				},
			},
			specVersion: *semver3_0_0,
			fail:        true,
			assertError: func(t *testing.T, err error) {
				errs := err.(multierror.Error)
				if assert.Len(t, errs, 2) {
					assert.Contains(t, errs[0].Error(), `"details.hostname" is undefined`)
					assert.ErrorIs(t, errs[1], errArrayOfObjects)
				}
			},
		},
		// elements in nested objects
		{
			key: "nested",
			value: []interface{}{
				map[string]interface{}{
					"id":       "somehost-id",
					"hostname": "somehost",
				},
			},
			definition: FieldDefinition{
				Name: "nested",
				Type: "nested",
				Fields: []FieldDefinition{
					{
						Name: "id",
						Type: "keyword",
					},
				},
			},
			specVersion: *semver3_0_0,
			fail:        true,
			assertError: func(t *testing.T, err error) {
				errs := err.(multierror.Error)
				if assert.Len(t, errs, 1) {
					assert.Contains(t, errs[0].Error(), `"nested.hostname" is undefined`)
				}
			},
		},
	} {

		t.Run(test.key, func(t *testing.T) {
			v := Validator{
				Schema:                       []FieldDefinition{test.definition},
				disabledDependencyManagement: true,
				enabledAllowedIPCheck:        true,
				allowedCIDRs:                 initializeAllowedCIDRsList(),
				specVersion:                  test.specVersion,
			}

			err := v.parseElementValue(test.key, test.definition, test.value, common.MapStr{})
			if test.fail {
				require.Error(t, err)
				if test.assertError != nil {
					test.assertError(t, err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompareKeys(t *testing.T) {
	cases := []struct {
		key         string
		def         FieldDefinition
		searchedKey string
		expected    bool
	}{
		{
			key:         "example.foo",
			searchedKey: "example.foo",
			expected:    true,
		},
		{
			key:         "example.bar",
			searchedKey: "example.foo",
			expected:    false,
		},
		{
			key:         "example.foo",
			searchedKey: "example.foos",
			expected:    false,
		},
		{
			key:         "example.foo",
			searchedKey: "example.fo",
			expected:    false,
		},
		{
			key:         "example.*",
			searchedKey: "example.foo",
			expected:    true,
		},
		{
			key:         "example.foo",
			searchedKey: "example.*",
			expected:    false,
		},
		{
			key:         "example.*",
			searchedKey: "example.",
			expected:    false,
		},
		{
			key:         "example.*.foo",
			searchedKey: "example.group.foo",
			expected:    true,
		},
		{
			key:         "example.*.*",
			searchedKey: "example.group.foo",
			expected:    true,
		},
		{
			key:         "example.*.*",
			searchedKey: "example..foo",
			expected:    false,
		},
		{
			key:         "example.*",
			searchedKey: "example.group.foo",
			expected:    false,
		},
		{
			key:         "example.geo",
			def:         FieldDefinition{Type: "geo_point"},
			searchedKey: "example.geo.lat",
			expected:    true,
		},
		{
			key:         "example.geo",
			def:         FieldDefinition{Type: "geo_point"},
			searchedKey: "example.geo.lon",
			expected:    true,
		},
		{
			key:         "example.geo",
			def:         FieldDefinition{Type: "geo_point"},
			searchedKey: "example.geo.foo",
			expected:    false,
		},
		{
			key:         "example.ecs.geo",
			def:         FieldDefinition{External: "ecs"},
			searchedKey: "example.ecs.geo.lat",
			expected:    true,
		},
		{
			key:         "example.ecs.geo",
			def:         FieldDefinition{External: "ecs"},
			searchedKey: "example.ecs.geo.lon",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "geo_point"},
			searchedKey: "example.geo.lon",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{External: "ecs"},
			searchedKey: "example.geo.lat",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "object", ObjectType: "geo_point"},
			searchedKey: "example.geo.lon",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "geo_point"},
			searchedKey: "example.geo.foo",
			expected:    false,
		},
		{
			key:         "example.histogram",
			def:         FieldDefinition{Type: "histogram"},
			searchedKey: "example.histogram.counts",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "histogram"},
			searchedKey: "example.histogram.counts",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "histogram"},
			searchedKey: "example.histogram.values",
			expected:    true,
		},
		{
			key:         "example.*",
			def:         FieldDefinition{Type: "histogram"},
			searchedKey: "example.histogram.foo",
			expected:    false,
		},
	}

	for _, c := range cases {
		t.Run(c.key+" matches "+c.searchedKey, func(t *testing.T) {
			found := compareKeys(c.key, c.def, c.searchedKey)
			assert.Equal(t, c.expected, found)
		})
	}
}

func TestValidateGeoPoint(t *testing.T) {
	validator, err := CreateValidatorForDirectory("../../test/packages/other/fields_tests/data_stream/first", WithDisabledDependencyManagement())

	require.NoError(t, err)
	require.NotNil(t, validator)

	e := readSampleEvent(t, "../../test/packages/other/fields_tests/data_stream/first/sample_event.json")
	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func TestValidateExternalMultiField(t *testing.T) {
	packageRoot := "../../test/packages/parallel/mongodb"
	dataStreamRoot := filepath.Join(packageRoot, "data_stream", "status")

	validator, err := createValidatorForDirectoryAndPackageRoot(dataStreamRoot,
		packageRootTestFinder{packageRoot})
	require.NoError(t, err)
	require.NotNil(t, validator)

	def := FindElementDefinition("process.name", validator.Schema)
	require.NotEmpty(t, def.MultiFields, "expected to test with a data stream with a field with multifields")

	e := readSampleEvent(t, "testdata/mongodb-multi-fields.json")
	var event common.MapStr
	err = json.Unmarshal(e, &event)
	require.NoError(t, err)

	v, err := event.GetValue("process.name.text")
	require.NotNil(t, v, "expected document with multi-field")
	require.NoError(t, err, "expected document with multi-field")

	errs := validator.ValidateDocumentBody(e)
	require.Empty(t, errs)
}

func readTestResults(t *testing.T, path string) (f results) {
	c, err := os.ReadFile(path)
	require.NoError(t, err)

	err = json.Unmarshal(c, &f)
	require.NoError(t, err)
	return
}

func readSampleEvent(t *testing.T, path string) json.RawMessage {
	c, err := os.ReadFile(path)
	require.NoError(t, err)
	return c
}
