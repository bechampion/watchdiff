# watchdiff 👀

A tiny CLI tool that watches a command and only prints what changed — with color.

No screen clearing, no full redraws. Just a clean stream of diffs.

## Install

```bash
go install github.com/jeronimog/watchdiff@latest
```

Or build from source:

```bash
git clone https://github.com/jeronimog/watchdiff.git
cd watchdiff
go build -o watchdiff .
sudo cp watchdiff /usr/local/bin/
```

## Usage

```bash
watchdiff [-n seconds] command
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-n` | Interval between runs (supports decimals) | `1` |

## Examples

### Watch database connections

```bash
watchdiff -n 0.5 'lsof -i tcp:5432 -n -P'
```

```
👀 watchdiff lsof -i tcp:5432 -n -P every 0.5s

🕐 14:32:07
  🔴 - main  1327053 jgarcia  12u  IPv6 4990021  TCP [::1]:45678->[::1]:5432 (ESTABLISHED)

🕐 14:32:15
  🟢 + main  1327053 jgarcia  14u  IPv6 4990088  TCP [::1]:45702->[::1]:5432 (ESTABLISHED)
```

### Watch pods disappearing

```bash
watchdiff -n 2 'kubectl get pods --no-headers'
```

```
👀 watchdiff kubectl get pods --no-headers every 2.0s

🕐 14:35:22
  🔴 - api-7b4f8c9d6-x2k9m   1/1   Running   0   4h
  🟢 + api-7b4f8c9d6-n8p3w   0/1   Pending   0   2s

🕐 14:35:28
  🔴 - api-7b4f8c9d6-n8p3w   0/1   Pending   0   8s
  🟢 + api-7b4f8c9d6-n8p3w   1/1   Running   0   8s
```

### Watch processes

```bash
watchdiff 'ps aux | grep postgres | grep -v grep'
```

```
👀 watchdiff ps aux | grep postgres | grep -v grep every 1.0s

🕐 14:40:11
  🔴 - postgres  1202672  0.0  0.1 220488 17280 ?  Ss  16:44  0:00 postgres: idle

🕐 14:40:45
  🟢 + postgres  1302454  0.0  0.1 220488 17280 ?  Ss  16:44  0:00 postgres: active
```

### Watch files in a directory

```bash
watchdiff -n 0.5 'ls -la /tmp/uploads/'
```

## How it works

1. Runs your command on a loop
2. Diffs the output against the previous run
3. Only prints lines that appeared or disappeared
4. Spins quietly between changes

🔴 Red = line disappeared  
🟢 Green = line appeared  

## License

MIT
