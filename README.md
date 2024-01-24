# bustime-scrape

The [NYCT bus GTFS static data](https://data.ny.gov/Transportation/MTA-General-Transit-Feed-Specification-GTFS-Static/fgm6-ccue/about_data) (at the time of this writing) appears to be missing many stops that are referenced on the [MTA Bus Time](https://bustime.mta.info/m/) website. This program scrapes all the stops and routes specified on the MTA Bus Time website and dumps them into CSVs in `out/`

## How to use

```
go run main.go
```
