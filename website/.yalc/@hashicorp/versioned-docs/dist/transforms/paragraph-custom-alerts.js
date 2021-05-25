"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.transformParagraphCustomAlerts = void 0;
const remark_1 = __importDefault(require("remark"));
const remark_mdx_1 = __importDefault(require("remark-mdx"));
const unist_util_is_1 = __importDefault(require("unist-util-is"));
const unist_util_visit_1 = __importDefault(require("unist-util-visit"));
const sigils = {
    '=>': 'success',
    '->': 'info',
    '~>': 'warning',
    '!>': 'danger',
};
// Embedding this here to remove reliance on `remark-html`
// TODO: update upstream plugin
const paragraphCustomAlertsPlugin = () => {
    return function transformer(tree) {
        unist_util_visit_1.default(tree, 'paragraph', (pNode, _, parent) => {
            let prevTextNode;
            unist_util_visit_1.default(pNode, 'text', (textNode) => {
                Object.keys(sigils).forEach((symbol) => {
                    // If this content has already been run through remark, -> will get escaped to \->
                    // and split into multiple text nodes, so we need to check for that
                    const isEscapedInfo = symbol === '->' &&
                        prevTextNode?.value === '-' &&
                        textNode.value.startsWith('> ');
                    if (textNode.value.startsWith(`${symbol} `) || isEscapedInfo) {
                        // Remove the literal sigil symbol from string contents
                        if (isEscapedInfo) {
                            prevTextNode.value = '';
                            textNode.value = textNode.value.replace('> ', '');
                        }
                        else {
                            textNode.value = textNode.value.replace(`${symbol} `, '');
                        }
                        // Wrap matched nodes with <div> (containing proper attributes)
                        parent.children = parent.children.map((node) => {
                            return unist_util_is_1.default(pNode, node)
                                ? {
                                    type: node.type,
                                    children: [
                                        {
                                            type: 'jsx',
                                            value: `<div className="alert alert-${sigils[symbol]} g-type-body">`,
                                        },
                                        {
                                            type: 'paragraph',
                                            children: [
                                                {
                                                    type: 'text',
                                                    value: '\n\n',
                                                },
                                                node,
                                                {
                                                    type: 'text',
                                                    value: '\n\n',
                                                },
                                            ],
                                        },
                                        {
                                            type: 'jsx',
                                            value: '</div>',
                                        },
                                    ],
                                }
                                : node;
                        });
                    }
                });
                prevTextNode = textNode;
            });
        });
    };
};
/**
 * Applying the paragraph-custom-alerts plugin here as some of the shortcodes are getting improperly escaped during stringification
 */
exports.transformParagraphCustomAlerts = {
    id: 'paragraph-custom-alerts',
    async transformer(document) {
        const contents = await remark_1.default()
            .use(paragraphCustomAlertsPlugin)
            .use(remark_mdx_1.default)
            .process(document.markdownSource);
        document.markdownSource = String(contents);
        return document;
    },
};
