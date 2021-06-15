const beautify = require('js-beautify');
const fs = require("fs");
const marked = require("marked");
const yaml = require('js-yaml');

const docMap = [{
    source: "jobs.mdx",
    target: "jobs-api.yaml",
    header: "List Jobs",
    path: "/jobs/get"
}];

docMap.forEach((item) =>{
    let md = fs.readFileSync(`./website/content/api-docs/${item.source}`, (error, data) => {
        if(error) {
            throw error;
        }
    }).toString();

    const tokens = marked.lexer(md);
    const headerVisitor = {};
    let currentHeader = '';

    tokens.forEach((token, index, array) => {
        // Will refactor this to use docMap
        // aggregate until next
        if (token.type === "heading" && token.depth === 2 && token.text !== currentHeader) {
            currentHeader = token.text;
            headerVisitor[currentHeader] =  [];
        } else if (currentHeader.length > 0) {
            headerVisitor[currentHeader] = headerVisitor[currentHeader].concat([token]);
        }
    });

    if (headerVisitor[item.header]) {
        try {
            let html = beautify.html(marked.parser(headerVisitor[item.header]));
            if (html.length < 1) {
                console.log(`error generating html for ${item.header}`);
                return;
            }

            let targetFile = `./openapi/v1/${item.target}`;
            let targetDoc = yaml.load(fs.readFileSync(targetFile, 'utf8'));

            let lastIndex = item.path.lastIndexOf('/');
            let path = item.path.substr(0, lastIndex);
            let method = item.path.substr(lastIndex, item.path.length - 1).replace('/', '');
            targetDoc.paths[path][method].description = '';

            let yamlStr = yaml.dump(targetDoc, {
                noRefs: true,
                replacer(key, value) {
                    if (key !== "description") return value;
                    return html;
                }
            });

            fs.writeFileSync(targetFile, yamlStr, 'utf8');
        } catch (e) {
            console.log(`error syncing ${item.path}: ${e}`);
        }
    }
});
