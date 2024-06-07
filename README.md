# metgo
A met.no library for go

## Features

* Covers the locationforecast 2.0 API call
* Integrated memory and disk cache
* Can be called as often as you want, the library takes care of fetching new data if needed or returning cached data otherwise

## Usage

The library is fairly simple to use. All you need to do is create a client (service) with a logger and then get your locationforecasts.
The rest is taken care of by the library.

Add the module:
```
go get github.com/roemer/metgo
```

Creation a logger and the service:
```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
metnoService, err := metgo.NewMetNoService("<sitename>", ".metno-cache", logger)
```
Make sure to set the sitename to the name of your website or application and some contact info.

Get data from the service:
```go
metnoData, err := metnoService.Locationforecast(lat, lon, altitude)
```

You then have the data object available in the returned variable.
