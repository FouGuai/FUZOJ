#!/usr/bin/env python3
import sys

def read_value(prompt: str) -> float:
    try:
        return float(input(prompt).strip())
    except EOFError:
        return float("nan")
    except ValueError:
        return float("nan")

def main() -> int:
    if len(sys.argv) >= 3:
        try:
            weight = float(sys.argv[1])
            height = float(sys.argv[2])
        except ValueError:
            print("Usage: bmi.py <weight_kg> <height_m>")
            return 2
    else:
        weight = read_value("Weight (kg): ")
        height = read_value("Height (m): ")

    if not (weight > 0 and height > 0):
        print("Invalid input. Weight and height must be positive numbers.")
        return 2

    bmi = weight / (height ** 2)
    print(f"BMI: {bmi:.2f}")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
