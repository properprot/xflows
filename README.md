<h1 align="center">XFlows</h1>

<div align="center">
    <img src="https://img.shields.io/github/go-mod/go-version/properprot/xflows?style=flat-square" alt="Go Version" />
    <img src="https://goreportcard.com/badge/github.com/properprot/xflows?style=flat-square" alt="Go Report Card" />
    <img src="https://img.shields.io/github/downloads/properprot/xflows/total?style=flat-square" alt="Downloads" />
</div>
<br>
<div align="center"><strong>X(DP)-based Flows</strong></div>
<div align="center">A SFlow agent designed for performance and ease of use.</div>

<div align="center">
    <h3>
        <a href="https://github.com/properprot/xflows/tree/main/.github/CONTRIBUTING.md">
            Contributing
        </a>
        <span> | </span>
        <a href="https://discord.gg/KFZ7ETWnAs">
            Discord
        </a>
        <span> | </span>
        <a href="https://github.com/properprot/xflows/tree/main/examples">
            Examples
        </a>
</div>

## Table of Contents
- [Features](#features)
- [Example](#example)
- [FAQ](#faq)

## Features
- __Config Based:__ Insanely easy to configure to any variety of use cases.
- __Performant:__ All aspects were designed with performance in mind.
- __Small API:__ With only a few methods, there's not much to learn.
- __Fault Tolerant:__ Can specify several outputs at once.

## Example
```go
exporter, err := xflows.NewExporter("config.yaml")
if err != nil {
    logger.WithError(err).Fatal("Failed to create XFlows exporter")
}

exporter.ListenFlows(program.SFlows)
```
Yes, it is literally that easy.

## FAQ
### Why did you do this?
Well, I got bored. Needed something to do so this was a good spend of 24 hours.

## Will this be fine in production?
Don't know til you've tried it but probably 🤷‍♂️
