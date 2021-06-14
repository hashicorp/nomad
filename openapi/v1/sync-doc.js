// const md = require('markdown-it')();
const args = process.argv.splice(2);
//
const docMap = [{
    source: "jobs.mdx",
    target: "jobs-api.yaml",
    header: "List Jobs",
    path: "/jobs/get"
}]
//
// console.log(md.render(args[0]));

const marked = require("marked");
const fs = require("fs");

// __dirname means relative to script. Use "./data.txt" if you want it relative to execution path.
// fs.readFile(__dirname + "/data.txt", (error, data) => {
//     if(error) {
//         throw error;
//     }
//     console.log(data.toString());
// });

md = args[0]
// console.log(markdown)

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

docMap.forEach((item) =>{
    let tokens = headerVisitor[item.header];

    if (tokens) {
        let html = marked.parser(tokens);
        console.log(html);
    }
});


// console.log(headerVisitor)



// const html = marked(args[0]);
// console.log(html)
