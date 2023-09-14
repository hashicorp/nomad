# Float Math Functions

All math functions operate on `float64` values.

## addf

Sum numbers with `addf`

This will return `5.5`:

```
addf 1.5 2 2
```

## add1f

To increment by 1, use `add1f`

## subf

To subtract, use `subf`

This is equivalent to `7.5 - 2 - 3` and will return `2.5`:

```
subf 7.5 2 3
```

## divf

Perform integer division with `divf`

This is equivalent to `10 / 2 / 4` and will return `1.25`:

```
divf 10 2 4
```

## mulf

Multiply with `mulf`

This will return `6`:

```
mulf 1.5 2 2
```

## maxf

Return the largest of a series of floats:

This will return `3`:

```
maxf 1 2.5 3
```

## minf

Return the smallest of a series of floats.

This will return `1.5`:

```
minf 1.5 2 3
```

## floor

Returns the greatest float value less than or equal to input value

`floor 123.9999` will return `123.0`

## ceil

Returns the greatest float value greater than or equal to input value

`ceil 123.001` will return `124.0`

## round

Returns a float value with the remainder rounded to the given number to digits after the decimal point.

`round 123.555555` will return `123.556`
