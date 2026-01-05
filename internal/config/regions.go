package config

type Region struct {
	Value       string `json:"value"`
	Text        string `json:"text"`
	HelpText    string `json:"help_text"`
	IconVariant string `json:"icon_variant,omitempty"`
}

// AWSRegions contains all AWS regions from the dashboard configuration
var AWSRegions = []Region{
	// USA
	{Value: "us-east-1", Text: "US East (N. Virginia)", HelpText: "us-east-1", IconVariant: "flag-US"},
	{Value: "us-east-2", Text: "US East (Ohio)", HelpText: "us-east-2", IconVariant: "flag-US"},
	{Value: "us-west-1", Text: "US West (N. California)", HelpText: "us-west-1", IconVariant: "flag-US"},
	{Value: "us-west-2", Text: "US West (Oregon)", HelpText: "us-west-2", IconVariant: "flag-US"},

	// Africa
	{Value: "af-south-1", Text: "Africa (Cape Town)", HelpText: "af-south-1", IconVariant: "flag-ZA"},

	// Asia
	{Value: "ap-east-1", Text: "Asia Pacific (Hong Kong)", HelpText: "ap-east-1", IconVariant: "flag-HK"},
	{Value: "ap-south-2", Text: "Asia Pacific (Hyderabad)", HelpText: "ap-south-2", IconVariant: "flag-IN"},
	{Value: "ap-southeast-3", Text: "Asia Pacific (Jakarta)", HelpText: "ap-southeast-3", IconVariant: "flag-ID"},
	{Value: "ap-southeast-4", Text: "Asia Pacific (Melbourne)", HelpText: "ap-southeast-4", IconVariant: "flag-AU"},
	{Value: "ap-south-1", Text: "Asia Pacific (Mumbai)", HelpText: "ap-south-1", IconVariant: "flag-IN"},
	{Value: "ap-northeast-3", Text: "Asia Pacific (Osaka)", HelpText: "ap-northeast-3", IconVariant: "flag-JP"},
	{Value: "ap-northeast-2", Text: "Asia Pacific (Seoul)", HelpText: "ap-northeast-2", IconVariant: "flag-KR"},
	{Value: "ap-southeast-1", Text: "Asia Pacific (Singapore)", HelpText: "ap-southeast-1", IconVariant: "flag-SG"},
	{Value: "ap-southeast-2", Text: "Asia Pacific (Sydney)", HelpText: "ap-southeast-2", IconVariant: "flag-AU"},
	{Value: "ap-northeast-1", Text: "Asia Pacific (Tokyo)", HelpText: "ap-northeast-1", IconVariant: "flag-JP"},

	// Canada
	{Value: "ca-central-1", Text: "Canada (Central)", HelpText: "ca-central-1", IconVariant: "flag-CA"},
	{Value: "ca-west-1", Text: "Canada West (Calgary)", HelpText: "ca-west-1", IconVariant: "flag-CA"},

	// Europe
	{Value: "eu-central-1", Text: "Europe (Frankfurt)", HelpText: "eu-central-1", IconVariant: "flag-DE"},
	{Value: "eu-west-1", Text: "Europe (Ireland)", HelpText: "eu-west-1", IconVariant: "flag-IE"},
	{Value: "eu-west-2", Text: "Europe (London)", HelpText: "eu-west-2", IconVariant: "flag-GB"},
	{Value: "eu-south-1", Text: "Europe (Milan)", HelpText: "eu-south-1", IconVariant: "flag-IT"},
	{Value: "eu-west-3", Text: "Europe (Paris)", HelpText: "eu-west-3", IconVariant: "flag-FR"},
	{Value: "eu-south-2", Text: "Europe (Spain)", HelpText: "eu-south-2", IconVariant: "flag-ES"},
	{Value: "eu-north-1", Text: "Europe (Stockholm)", HelpText: "eu-north-1", IconVariant: "flag-SE"},
	{Value: "eu-central-2", Text: "Europe (Zürich)", HelpText: "eu-central-2", IconVariant: "flag-CH"},

	// Israel
	{Value: "il-central-1", Text: "Israel (Tel Aviv)", HelpText: "il-central-1", IconVariant: "flag-IL"},

	// Middle East
	{Value: "me-south-1", Text: "Middle East (Bahrain)", HelpText: "me-south-1", IconVariant: "flag-BH"},
	{Value: "me-central-1", Text: "Middle East (UAE)", HelpText: "me-central-1", IconVariant: "flag-AE"},

	// South America
	{Value: "sa-east-1", Text: "South America (São Paulo)", HelpText: "sa-east-1", IconVariant: "flag-BR"},
}

// AzureRegions contains common Azure regions from the dashboard configuration
var AzureRegions = []Region{
	{Value: "eastus", Text: "East US", HelpText: "(US) East US", IconVariant: "flag-US"},
	{Value: "eastus2", Text: "East US 2", HelpText: "(US) East US 2", IconVariant: "flag-US"},
	{Value: "southcentralus", Text: "South Central US", HelpText: "(US) South Central US", IconVariant: "flag-US"},
	{Value: "westus2", Text: "West US 2", HelpText: "(US) West US 2", IconVariant: "flag-US"},
	{Value: "westus3", Text: "West US 3", HelpText: "(US) West US 3", IconVariant: "flag-US"},
	{Value: "australiaeast", Text: "Australia East", HelpText: "(Asia Pacific) Australia East", IconVariant: "flag-AU"},
	{Value: "southeastasia", Text: "Southeast Asia", HelpText: "(Asia Pacific) Southeast Asia", IconVariant: "flag-SG"},
	{Value: "northeurope", Text: "North Europe", HelpText: "(Europe) North Europe", IconVariant: "flag-IE"},
	{Value: "swedencentral", Text: "Sweden Central", HelpText: "(Europe) Sweden Central", IconVariant: "flag-SE"},
	{Value: "uksouth", Text: "UK South", HelpText: "(Europe) UK South", IconVariant: "flag-GB"},
	{Value: "westeurope", Text: "West Europe", HelpText: "(Europe) West Europe", IconVariant: "flag-NL"},
	{Value: "centralus", Text: "Central US", HelpText: "(US) Central US", IconVariant: "flag-US"},
	{Value: "southafricanorth", Text: "South Africa North", HelpText: "(Africa) South Africa North", IconVariant: "flag-ZA"},
	{Value: "centralindia", Text: "Central India", HelpText: "(Asia Pacific) Central India", IconVariant: "flag-IN"},
	{Value: "eastasia", Text: "East Asia", HelpText: "(Asia Pacific) East Asia", IconVariant: "flag-HK"},
	{Value: "japaneast", Text: "Japan East", HelpText: "(Asia Pacific) Japan East", IconVariant: "flag-JP"},
	{Value: "koreacentral", Text: "Korea Central", HelpText: "(Asia Pacific) Korea Central", IconVariant: "flag-KR"},
	{Value: "canadacentral", Text: "Canada Central", HelpText: "(Canada) Canada Central", IconVariant: "flag-CA"},
	{Value: "francecentral", Text: "France Central", HelpText: "(Europe) France Central", IconVariant: "flag-FR"},
	{Value: "germanywestcentral", Text: "Germany West Central", HelpText: "(Europe) Germany West Central", IconVariant: "flag-DE"},
	{Value: "norwayeast", Text: "Norway East", HelpText: "(Europe) Norway East", IconVariant: "flag-NO"},
	{Value: "switzerlandnorth", Text: "Switzerland North", HelpText: "(Europe) Switzerland North", IconVariant: "flag-CH"},
	{Value: "uaenorth", Text: "UAE North", HelpText: "(Middle East) UAE North", IconVariant: "flag-AE"},
	{Value: "brazilsouth", Text: "Brazil South", HelpText: "(South America) Brazil South", IconVariant: "flag-BR"},
}
