package main

import "embed"

//go:embed config/*.json sample.json native_sample.json fonts/*
var assets embed.FS



