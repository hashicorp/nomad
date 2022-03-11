package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"net/http"
	"strings"
	"time"
)

type climatiqProvider struct {
	config *EnergyConfig
}

type CloudProviderConfig struct {
	Name    string   `hcl:"name,label"`
	Regions []string `hcl:"regions"`
}

func (cpc *CloudProviderConfig) Validate() error {
	if cpc == nil {
		return fmt.Errorf("invalid energy config: Climatiq specified but no cloud_provider configured")
	}

	if cpc.Name == "" {
		return fmt.Errorf("invalid energy config: cloud_provider name required")
	}

	return nil
}

type ClimatiqConfig struct {
	CloudProviders []CloudProviderConfig `hcl:"cloud_provider, block"`
	APIUrl         string                `hcl:"api_url"`
	APIKey         string                `hcl:"api_key"`
}

func (cl *ClimatiqConfig) Copy() *ClimatiqConfig {
	if cl == nil {
		return nil
	}

	n := new(ClimatiqConfig)
	*n = *cl

	return n
}

func (cl *ClimatiqConfig) Validate() error {
	if cl == nil {
		return fmt.Errorf("invalid energy config: Climatiq specified but not configured")
	}

	var mErr multierror.Error

	if cl.APIKey == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid energy config: api_key required"))
	}

	if cl.APIUrl == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid energy config: api_url required"))
	}

	if len(cl.CloudProviders) == 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid energy config: cloud_providers required"))
	}

	for _, p := range cl.CloudProviders {
		err := p.Validate()
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}

	return mErr.ErrorOrNil()
}

func newClimatiqProvider(config *EnergyConfig) (EnergyScoreProvider, error) {
	return &climatiqProvider{
		config,
	}, nil
}

var postDataTemplate = `{
    "cpu_count": 1,
    "region": "{region}",
    "cpu_load": 1,
    "duration": 1,
    "duration_unit": "h"
}`

func (cq *climatiqProvider) GetCarbonIntensity(ctx context.Context) (int, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	respObj, err := cq.getRegionResponse(ctx, client, cq.config.Region)
	if err != nil {
		return 0, err
	}

	if respObj.Co2E == 0 {
		return 0, fmt.Errorf("climatiq failed to get response for node region")
	}

	nodeRegionScore := respObj.Co2E

	regionScores := make(map[string]float64, len(cq.config.ClimatiqConfig.CloudProviders))
	regionScores[cq.config.Region] = nodeRegionScore

	for _, v := range cq.config.ClimatiqConfig.CloudProviders {
		for _, region := range v.Regions {
			if region == cq.config.Region {
				continue
			}

			regionScore, err := cq.getRegionResponse(ctx, client, region)
			if err != nil {
				return 0, err
			}

			regionScores[region] = regionScore.Co2E
		}
	}

	min := -1.0
	for _, v := range regionScores {
		if min == -1 {
			min = v
		} else if v < min {
			min = v
		}
	}
	max := -1.0
	for _, v := range regionScores {
		if max == -1 {
			max = v
		} else if v > max {
			max = v
		}
	}

	normalized := normalize(min, max, nodeRegionScore)

	return normalized, nil
}

func (cq *climatiqProvider) getRegionResponse(ctx context.Context, client *http.Client, region string) (*ClimatiqResponse, error) {
	// Naively just doing one for now
	apiURL := strings.Replace(cq.config.ClimatiqConfig.APIUrl, "{provider}", cq.config.ClimatiqConfig.CloudProviders[0].Name, -1)

	reqObj := &ClimatiqRequest{
		CpuCount:     1,
		Region:       region,
		CpuLoad:      1,
		Duration:     1,
		DurationUnit: "h",
	}

	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(reqObj)
	if err != nil {
		return nil, err
	}
	var bearer = "Bearer " + cq.config.ClimatiqConfig.APIKey

	fmt.Println(b.String())

	req, err := http.NewRequest("POST", apiURL, b)
	if err != nil {
		return nil, err
	}
	//req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", bearer)

	resp, err := client.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		body := buf.String()
		return nil, fmt.Errorf("%s: %s", resp.Status, body)
	}

	respObj := &ClimatiqResponse{}

	err = json.NewDecoder(resp.Body).Decode(respObj)
	if err != nil {
		return nil, err
	}
	return respObj, nil
}

func (cq *climatiqProvider) RecommendRegion() (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	regionScores := make(map[string]float64, len(cq.config.ClimatiqConfig.CloudProviders))

	//for _, v := range cq.config.ClimatiqConfig.CloudProviders {
	for _, region := range climatiqAWSRegions { //} v.Regions {
		if region == cq.config.Region {
			continue
		}

		regionScore, err := cq.getRegionResponse(nil, client, region)
		if err != nil {
			return "", err
		}

		regionScores[region] = regionScore.Co2E
	}
	//}

	result := ""
	minScore := 0.0
	for region, score := range regionScores {
		if result == "" {
			result = region
			minScore = score
			continue
		}

		if score < minScore {
			minScore = score
			result = region
		}
	}

	return result, nil
}

type ClimatiqResponse struct {
	Co2E                  float64 `json:"co2e"`
	Co2EUnit              string  `json:"co2e_unit"`
	Co2ECalculationMethod string  `json:"co2e_calculation_method"`
	Co2ECalculationOrigin string  `json:"co2e_calculation_origin"`
	EmissionFactor        struct {
		Id          string `json:"id"`
		Source      string `json:"source"`
		Year        string `json:"year"`
		Region      string `json:"region"`
		Category    string `json:"category"`
		LcaActivity string `json:"lca_activity"`
	} `json:"emission_factor"`
	ConstituentGases struct {
		Co2ETotal float64     `json:"co2e_total"`
		Co2EOther interface{} `json:"co2e_other"`
		Co2       interface{} `json:"co2"`
		Ch4       interface{} `json:"ch4"`
		N2O       interface{} `json:"n2o"`
	} `json:"constituent_gases"`
}

type ClimatiqRequest struct {
	CpuCount     int    `json:"cpu_count"`
	Region       string `json:"region"`
	CpuLoad      int    `json:"cpu_load"`
	Duration     int    `json:"duration"`
	DurationUnit string `json:"duration_unit"`
}

var climatiqAWSRegions = []string{
	"af_south_1",
	"ap_east_1",
	"ap_northeast_1",
	"ap_northeast_2",
	"ap_northeast_3",
	"ap_south_1",
	"ap_southeast_1",
	"ap_southeast_2",
	"ca_central_1",
	"cn_north_1",
	"cn_northwest_1",
	"eu_central_1",
	"eu_north_1",
	"eu_south_1",
	"eu_west_1",
	"eu_west_2",
	"eu_west_3",
	"me_south_1",
	"sa_east_1",
	"us_east_1",
	"us_east_2",
	"us_gov_east_1",
	"us_gov_west_1",
	"us_west_1",
	"us_west_2",
}
