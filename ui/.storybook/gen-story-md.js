#!/usr/bin/env node
/* eslint-disable */
// run this script via yarn in the ui directory:
// yarn gen-story-md some-component

const fs = require('fs');
const jsdoc2md = require('jsdoc-to-markdown');
var args = process.argv.slice(2);
const name = args[0];

const inputFile = `app/components/${name}.js`;
const outputFile = `stories/components/${name}.md`;

const component = name
  .split('-')
  .map(word => word.charAt(0).toUpperCase() + word.slice(1))
  .join('');
const options = {
  files: inputFile,
  template: fs.readFileSync('./.storybook/story-md.hbs', 'utf8'),
  'example-lang': 'js',
};
let md = jsdoc2md.renderSync(options);

const pageBreakIndex = md.lastIndexOf('---'); //this is our last page break

const seeLinks = `**See**

- [uses of \`${component}\`](https://github.com/hashicorp/nomad/search?l=Handlebars&q=${component}+OR+${name})
- [\`${component}\` source](https://github.com/hashicorp/nomad/blob/master/ui/${inputFile})

`;
const generatedWarning = `<!--THIS FILE IS AUTO GENERATED. This file is generated from JSDoc comments in ${inputFile}. To make changes, first edit that file and run "yarn gen-story-md ${name}" to re-generate the content.-->

`;
md = generatedWarning + md.slice(0, pageBreakIndex) + seeLinks + md.slice(pageBreakIndex);

fs.writeFileSync(outputFile, md);
