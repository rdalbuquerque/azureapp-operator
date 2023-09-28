$configs = Get-Content ".env"
foreach ($config in $configs) {
    $envName = $config.Split("=")[0]
    $envValue = $config.Split("=")[1].Trim('"')
    [System.Environment]::SetEnvironmentVariable($envName, $envValue)
}
