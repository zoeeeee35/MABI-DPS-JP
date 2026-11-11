package main

import "embed"

//go:embed config/*.json sample.json native_sample.json modern-ui/* fonts/*
var assets embed.FS


