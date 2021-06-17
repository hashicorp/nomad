const beautify = require('js-beautify');
const fs = require("fs");
const marked = require("marked");
const yaml = require('js-yaml');
const docMap = require('./doc-map.js');

// First sync index page with top level API description
let indexPage = fs.readFileSync('./website/content/api-docs/index.mdx', (error, data) => {
    if(error) {
        throw error;
    }
}).toString();

// strip document front matter if exists
let tokens = marked.lexer(indexPage);
let frontMatterEnd = tokens.map((t) => { return t.raw; }).indexOf('---\n\n') + 1; // add one to get next line
if (frontMatterEnd > 0) {
    tokens = tokens.slice(frontMatterEnd);
}

// Write main api index to base info description.
let indexHtml = beautify.html(marked.parser(tokens));
let baseFile = './openapi/v1/base.yaml';
let baseDoc = yaml.load(fs.readFileSync(baseFile, 'utf8'));
baseDoc.info.description = indexHtml;

fs.writeFileSync(baseFile, yaml.dump(baseDoc, {
    noRefs: true
}), 'utf8');

// Next sync documentation in the document map with the path descriptions.
// Iterates of docMap and ensures documentation consistency between API docs on
// io website and combined OpenAPI specification.
docMap.forEach((item) =>{
    // Get the markdown from the source document
    let md = fs.readFileSync(`./website/content/api-docs/${item.source}`, (error, data) => {
        if(error) {
            throw error;
        }
    }).toString();

    // tokenize the document
    const tokens = marked.lexer(md);
    let mapItem = undefined;
    tokens.forEach((token) => {
        // set mapItem when transitioning to new section
        if (token.type === "heading" && token.depth === 2) {
            mapItem = item.map.find((i) => {
                return i.header === token.text;
            });
        } // if mapItem add all the tokens that are in this section to the visitor
        else if (mapItem) {
            mapItem.tokens = mapItem.tokens.concat([token]);
        }
    });

    // Insert the html to the target document by section then write file.
    try {
        let targetFile = `./openapi/v1/${item.target}`;
        let targetDoc = yaml.load(fs.readFileSync(targetFile, 'utf8'));

        item.map.forEach((mappedItem) => {
            let html = beautify.html(marked.parser(mappedItem.tokens));
            if (html.length < 1) {
                console.log(`error generating html for ${mappedItem.header}`);
                return;
            }

            let lastIndex = mappedItem.path.lastIndexOf('/');
            let path = mappedItem.path.substr(0, lastIndex);
            let method = mappedItem.path.substr(lastIndex, mappedItem.path.length - 1).replace('/', '');
            targetDoc.paths[path][method].description = html;
        });

        let yamlDump = yaml.dump(targetDoc, {
            noRefs: true
        });

        fs.writeFileSync(targetFile, yamlDump, 'utf8');
    } catch (e) {
        console.log(`error syncing ${mappedItem.path}: ${e}`);
    }
});

let jobsApi = yaml.load(fs.readFileSync('./openapi/v1/jobs-api.yaml', 'utf8'));
let schemas = yaml.load(fs.readFileSync('./openapi/v1/schemas.yaml', 'utf8'));

let specDoc = {
    ...baseDoc
}

specDoc.components = {
    ...specDoc.components,
    ...schemas.components
}

specDoc.paths = {
    ...specDoc.paths,
    ...jobsApi.paths
}

let specDump = yaml.dump(specDoc, {
    noRefs: true
});

fs.writeFileSync('./openapi/v1/openapi.yaml', specDump, 'utf8');