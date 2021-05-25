"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.makeServeStaticAssets = void 0;
function makeServeStaticAssets(product) {
    return async function serveStaticAssets(req, res) {
        const image = await fetch(`https://raw.githubusercontent.com/hashicorp/${product}/${req.query.version}/website/public/${req.query.asset.join('/')}`);
        // @ts-expect-error
        const buffer = await image.buffer();
        const contentType = image.headers.get('content-type');
        if (contentType) {
            res.setHeader('content-type', contentType);
        }
        res.end(buffer);
    };
}
exports.makeServeStaticAssets = makeServeStaticAssets;
