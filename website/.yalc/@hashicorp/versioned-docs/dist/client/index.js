"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.VersionSelect = exports.removeVersionFromPath = exports.normalizeVersion = exports.getVersionFromPath = void 0;
const react_1 = __importDefault(require("react"));
const router_1 = require("next/router");
const react_select_input_1 = __importDefault(require("@hashicorp/react-select-input"));
const util_1 = require("../util");
Object.defineProperty(exports, "getVersionFromPath", { enumerable: true, get: function () { return util_1.getVersionFromPath; } });
function normalizeVersion(version) {
    return version.startsWith('v') ? version : `v${version}`;
}
exports.normalizeVersion = normalizeVersion;
/**
 * Remove the version segment from a path string, or an array of path segments
 */
function removeVersionFromPath(path = []) {
    let pathSegments = path;
    if (!Array.isArray(pathSegments))
        pathSegments = pathSegments.split('/');
    const versionIndex = pathSegments.findIndex((el) => /^v\d+\.\d/.test(el));
    if (versionIndex !== -1) {
        pathSegments.splice(versionIndex, 1);
    }
    return pathSegments.join('/');
}
exports.removeVersionFromPath = removeVersionFromPath;
/**
 * Component which accepts a list of versions and renders a select component. Navigates to the new version on select
 */
function VersionSelect({ versions, }) {
    const router = router_1.useRouter();
    const versionInPath = util_1.getVersionFromPath(router.asPath);
    const onVersionSelect = (newVersion) => {
        const remove = versionInPath ? 1 : 0;
        const pathParts = router.asPath.split('/');
        if (newVersion === 'latest' && versionInPath) {
            pathParts.splice(2, remove);
        }
        else {
            pathParts.splice(2, remove, newVersion);
        }
        router.push(pathParts.join('/'));
    };
    const selectedVersion = versions.find((ver) => ver.name === versionInPath) || versions[0];
    return (react_1.default.createElement(react_select_input_1.default, { size: "small", options: versions, value: selectedVersion, defaultLabel: "Version", onValueChange: onVersionSelect, label: "Version" }));
}
exports.VersionSelect = VersionSelect;
