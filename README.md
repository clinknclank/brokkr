# Brokkr
![Coverage](https://img.shields.io/badge/Coverage-81.4%25-brightgreen)
![GitHub Super-Linter](https://github.com/clinknclank/brokkr/actions/workflows/lint.yml/badge.svg)

## About Brokkr
> Brokkr provides a core entry point to manage the lifecycle of applications. 
> However code structure based on composition and focus is to provide slim components that can be used independently.
> 
> In other words, focus on value - build prototypes, apps and API's. 

### Goals
Boosts your productivity by using reusable components in your project.

### Principles
- KISS - Keep it simple, stupid.
- Don't repeat yourself - use what can be re-used.
- Composition

## Contribute

### Getting Started

Create `Brokkr` entry point to manage the lifecycle, here simple bootstrap example:

```go
package main

import (
	"github.com/clinknclank/brokkr"
)

func main() {
    // It's up to you how you load your configuration
    // yourAPIConfigurationStructure
    
    // Maybe you have gRPC or HTTP server that must be managed.
    // yourServer
    
    // And maybe you have metric collector that must be executed in background besides `yourServer`
    // yourMetricCollectorWatcher
    
    opts := []brokkr.Options{
        brokkr.SetForceStopTimeout(yourAPIConfigurationStructure.ServerShutdownTimeout), // Passing timeout to stop Brokkr.
        brokkr.AddBackgroundTasks(yourServer), // Passing your server.
        brokkr.AddBackgroundTasks(yourMetricCollectorWatcher), // Passing your metrics collector background process.
    }
    
    myApp := brokkr.NewBrokkr(opts...)
	
	// Execute your app that will have `yourServer` and `yourMetricCollectorWatcher`
	// in controlled gorutine as background process, which will be managed by same main loop inside `Brokkr`.
	if err := myApp.Start(); err != nil {
		panic(err)
	}
}
```

### Run tests

```bash
go test -v --race $(go list ./... | (grep -v /vendor/) | (grep -v internal/test/bdd/integration_tests))
```
