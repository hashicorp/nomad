# Subnav

Displays a navigation bar, with links and a call-to-action. Links can be organized into submenus, although from a UX standpoint it's preferable to provide simplified, flattened navigation. The Subnav is intended for use directly below our primary navigation.

## Generating a new subnav/icon File

### Generate the SVGR File

Gather the .svg logo of the new product that we want to include as an option in the Hero and run it through svgr:

```sh
npx @svgr/cli --icon boundary-logo-color.svg > boundary-logo-color.svgr.js
```

This will yield a file like this:

```jsx
import * as React from 'react'

function SvgBoundaryLogoColor(props) {
  return (
    <svg viewBox="0 0 310 67" width="1em" height="1em" {...props}>
      <defs>
        <style>{'.boundary-logo-color_svg__cls-1{fill:var(--boundary)}'}</style>
      </defs>
      <g id="boundary-logo-color_svg__LOGOS">
        <path d="M104.58 32.17c3.08-1.33 4.48-3.21 4.48-6.54v-4.18c0-5.44-3.09-8.59-10.72-8.59H82.5v40.3h16.94c7.32 0 11-3.57 11-9.26v-4.29c.01-3.94-2.17-6.61-5.86-7.44zm-14.46-13h7.32c2.78 0 4 1 4 3.27V26c0 2.36-1.08 3.76-4.05 3.76h-7.27zm12.71 23.78c0 3-1.46 3.94-4.91 3.94h-7.8v-11h8.47c3 0 4.24 1.33 4.24 4zM126.48 23.09c-10.1 0-12.82 5.56-12.82 11.62v7.44c0 6.05 2.72 11.61 12.82 11.61s12.83-5.56 12.83-11.61v-7.44c0-6.06-2.73-11.62-12.83-11.62zm5.45 19.3c0 3.33-1.51 5.08-5.45 5.08S121 45.72 121 42.39v-7.92c0-3.34 1.51-5.09 5.44-5.09s5.45 1.75 5.45 5.09zM161.46 23.69v20.46c-2.67 1.39-5.88 2.48-7.69 2.48s-2.36-.79-2.36-2.37V23.69H144v21.36c0 5.27 1.75 8.71 6.65 8.71a29.59 29.59 0 0011.8-3.08l.73 2.48h5.63V23.69zM193.34 23.09a29.59 29.59 0 00-11.8 3.08l-.72-2.48h-5.63v29.47h7.38V32.71c2.66-1.39 5.87-2.48 7.68-2.48s2.36.79 2.36 2.36v20.57H200V31.8c0-5.26-1.76-8.71-6.66-8.71zM222.44 11.66v12.28a41 41 0 00-7.87-.85c-6.83 0-9.73 3.87-9.73 10.41v10c0 6.72 3.14 10.22 9.07 10.22a15.48 15.48 0 009.32-3.08l.72 2.48h5.87V10.63zm0 33.15a10.43 10.43 0 01-6.59 2.66c-2.73 0-3.63-1.33-3.63-3.75v-10.4c0-2.67 1-3.94 3.69-3.94a33 33 0 016.53.79zM245.74 23.09a38.76 38.76 0 00-10.11 1.39l.91 5.63a40.74 40.74 0 018.17-.91c4.71 0 5.62 1.15 5.62 4.42v3.08h-7.07c-6 0-8.6 2.3-8.6 8.29 0 5.09 2.3 8.77 7.69 8.77a16.06 16.06 0 008.77-2.6l.54 2h6.06V33.74c0-7.39-2.72-10.65-11.98-10.65zm4.59 23.29a11.57 11.57 0 01-5.56 1.51c-2.3 0-3-.9-3-3 0-2.24.67-2.9 3.09-2.9h5.44zM277.92 23.09a33 33 0 00-7.74 3.91l-.48-3.27h-6.24v29.43h7.39v-19a56 56 0 017.8-4.29zM300.61 23.69l-6.77 22.39-6.72-22.39h-7.56l9.32 29.47h2.59l-3.68 11.92h7l3.93-11.92 9.44-29.47z" />
        <path
          className="boundary-logo-color_svg__cls-1"
          d="M9.89 64.1v-4.82H19.9V56.3h-4.85v-4.85h22.92L27.32 33.01l10.65-18.44h-23.5v25.58H1.82V1.92h43.52l7.27 12.58-10.69 18.51L52.83 51.9l-7.05 12.2H9.89z"
        />
        <path
          className="boundary-logo-color_svg__cls-1"
          d="M6.98 51.45h4.85v4.85H6.98zM1.82 59.25h4.85v4.85H1.82z"
        />
      </g>
    </svg>
  )
}

export default SvgBoundaryLogoColor
```

### Adjust the width & height

The width & height probably was output incorrectly. Update the folowing below:

```diff
- <svg viewBox="0 0 310 67" width="1em" height="1em" {...props}>
+ <svg viewBox="0 0 310 67" width={310} height={67} {...props}>
```

### Wrap it in a g-svg Div

The entire output component needs to be updated and wrapped in a `g-svg` div.

```jsx
<div className="g-svg"></div>
```

### Add an Img sizer block

The first child of the above `g-svg` wrapper must contain a spacer. The sizer is a blank PNG with the same size as the specified SVG viewbox.

```jsx
<img src="OUTPUT-FROM-BELOW" className="svg-sizer" alt="" role="presentation" />
```

To get the proper value of `src`, run the following commands.

```sh
# Generate the Stub Image (requires imgmagick -- brew install imagemagick)
# TODO, Replace 310x67 with the appropriate sizing for the SVG
convert -size 310x67 xc:transparent sizer.png

# Get the Base64ified version of it
echo -n "data:image/png;base64,"; cat sizer.png | base64
```
