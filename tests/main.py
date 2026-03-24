import sys


def main() -> int:
    data = sys.stdin.read().strip().split()
    if len(data) < 2:
        return 0
    a = int(data[0])
    b = int(data[1])
    sys.stdout.write(f"{a + b}\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
