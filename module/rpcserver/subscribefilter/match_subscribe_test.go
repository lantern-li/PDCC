/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper base test
package subscribefilter

import "testing"

// TestGetIdentityCode test GetIdentityCode
func TestGetIdentityCode(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectError       bool
		expectedCodeType  string
		expectedProvince  string
		expectedCity      string
		expectedDistrict  string
		expectedExtension string
	}{
		{
			name:              "Valid 11-digit code",
			input:             "12345678901",
			expectError:       false,
			expectedCodeType:  "1",
			expectedProvince:  "23",
			expectedCity:      "45",
			expectedDistrict:  "67",
			expectedExtension: "8901",
		},
		{
			name:              "Valid 11-digit code with spaces",
			input:             "1 23 45 67 8901",
			expectError:       false,
			expectedCodeType:  "1",
			expectedProvince:  "23",
			expectedCity:      "45",
			expectedDistrict:  "67",
			expectedExtension: "8901",
		},
		{
			name:              "Valid 19-digit code",
			input:             "1234567890123456789",
			expectError:       false,
			expectedCodeType:  "1",
			expectedProvince:  "23",
			expectedCity:      "45",
			expectedDistrict:  "67",
			expectedExtension: "890123456789",
		},
		{
			name:        "Invalid length",
			input:       "1234567",
			expectError: true,
		},
		{
			name:              "Invalid format (non-digit character)",
			input:             "1a345678901",
			expectError:       false,
			expectedCodeType:  "1",
			expectedProvince:  "a3",
			expectedCity:      "45",
			expectedDistrict:  "67",
			expectedExtension: "8901",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ic, err := GetIdentityCode(tc.input)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected an error for input %q but got none", tc.input)
				}
			} else {
				if err != nil {
					t.Fatalf("did not expect an error for input %q but got: %v", tc.input, err)
				}
				if ic.CodeType != tc.expectedCodeType {
					t.Errorf("expected CodeType %q, got %q", tc.expectedCodeType, ic.CodeType)
				}
				if ic.ProvinceCode != tc.expectedProvince {
					t.Errorf("expected ProvinceCode %q, got %q", tc.expectedProvince, ic.ProvinceCode)
				}
				if ic.CityCode != tc.expectedCity {
					t.Errorf("expected CityCode %q, got %q", tc.expectedCity, ic.CityCode)
				}
				if ic.DistrictCode != tc.expectedDistrict {
					t.Errorf("expected DistrictCode %q, got %q", tc.expectedDistrict, ic.DistrictCode)
				}
				if ic.Extension != tc.expectedExtension {
					t.Errorf("expected Extension %q, got %q", tc.expectedExtension, ic.Extension)
				}
			}
		})
	}
}
