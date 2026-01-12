# Showdown - Scrum Poker Game

Showdown is a collaborative estimation tool written in Go and used in
Agile/Scrum development where team members vote on story point estimates during
sprint planning. The name "Showdown" refers to the final reveal in a poker game,
similar to how votes are revealed simultaneously in planning poker.

It runs as a SSH server (default port `23234`) that team members can connect to
using their terminal application. Scrum Master is authorized via SSH key
(`.ssh/showdown_keys`), and controls the game, reveals votes and resets rounds.
Each player can connect (without a key) to the game and select storypoints.

## Demo

![Showdown view of Scrum Master](./images/showdown-master.gif)

![Showdown view of team player](./images/showdown-player.gif)

## Why

I have created this to learn more about Go and I wanted to work on a project that can be used in a terminal. The Go
modules from [Charm](https://charm.sh) looked very nice and easy to use.

The best way to learn a new programming language is to define a project that you want to build and start working on it!

## Who is it for?

This project is my personal project, although you can and may contribute to it, fork it, or whatever.

## How to use

To use Showdown:

```bash
$ showdown
2024/11/15 10:01:22 INFO Starting Scrum Poker server host=Beans-with-Bacon-Megarocket.local port=23234
```

Default port is`23234`, but you can overwrite this with the option `-p`.

```bash
$ showdown -p 2222
2024/11/15 10:02:55 INFO Starting Scrum Poker server host=Beans-with-Bacon-Megarocket.local port=2222
```
