"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    Object.defineProperty(o, k2, { enumerable: true, get: function() { return m[k]; } });
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (k !== "default" && Object.prototype.hasOwnProperty.call(mod, k)) __createBinding(result, mod, k);
    __setModuleDefault(result, mod);
    return result;
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.writeVersionManifest = exports.loadVersionManifest = void 0;
const fs = __importStar(require("fs"));
const path = __importStar(require("path"));
async function loadVersionManifest(cwd = process.cwd()) {
    let result = [];
    try {
        result = JSON.parse(await fs.promises.readFile(path.join(cwd, 'version-manifest.json'), {
            encoding: 'utf-8',
        }));
    }
    catch { }
    return result;
}
exports.loadVersionManifest = loadVersionManifest;
async function writeVersionManifest(manifest, cwd = process.cwd()) {
    await fs.promises.writeFile(path.join(cwd, 'version-manifest.json'), JSON.stringify(manifest, null, 2) + '\n', { encoding: 'utf-8' });
}
exports.writeVersionManifest = writeVersionManifest;
