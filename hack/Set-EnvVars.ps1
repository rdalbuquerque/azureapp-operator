<#
This script expects a .env json file in the following format
{
    "envvar1": "value1",
    "envvar2": "value2",
    ...
}
#>
$envvars = Get-Content ".env.json" | ConvertFrom-Json
foreach ($var in $envvars.psobject.Properties) {
    $envName = $var.Name
    $envValue = $var.Value
    [System.Environment]::SetEnvironmentVariable($envName, $envValue)
}
