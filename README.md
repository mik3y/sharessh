# ShareSSH

A simple tool to temporarily share SSH access to your machine, to a specific GitHub user.

## Usage

```
$ sharessh <github username>
```

## Why?

If you're building an open source project, sometimes your users might run into bugs where the fastest way to understand them is, "Hey would you be open to letting me SSH in and check it out?"

This may be particularly the case with hardware or IoT projects, where it's not as easy to remotely debug.

I ran into this situation and remembered GitHub exposes public keys as `https://github.com/<username>.keys`. I thought it would be cool to write a little program to _temporarily_ open up an SSH server just for one user's keys. _"Hey would you mind running `sharessh mik3y` so I can check things out?".

## Warning

Connecting users will have shell access for whatever user runs this command.

## Contributing

Contributions welcome! Open an issue or a pull request if you've got ideas.
