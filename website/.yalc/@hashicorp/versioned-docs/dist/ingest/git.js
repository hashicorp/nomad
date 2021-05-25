"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getGitSha = exports.checkoutRefAndExecute = void 0;
const util_1 = require("util");
const execFile = util_1.promisify(require('child_process').execFile);
/**
 * Checkout the provided git ref and execute the provided fn in the context of that ref
 */
async function checkoutRefAndExecute(ref, fn) {
    if (process.env.NODE_ENV === 'test') {
        await fn();
        return;
    }
    let gitStashApplied = false;
    // Ensure the working tree is in a clean state so we can checkout
    // TODO: is this too invasive?
    try {
        await execFile('git', ['stash']);
        gitStashApplied = true;
    }
    catch { }
    try {
        await execFile('git', ['fetch', 'origin', ref]);
        await execFile('git', ['checkout', 'FETCH_HEAD']);
    }
    catch (error) {
        console.log(`Error checking out ref '${ref}'`, error);
        if (gitStashApplied)
            await execFile('git', ['stash', 'pop']);
        return;
    }
    try {
        await fn();
    }
    finally {
        // reset the checked out ref to what it was previously
        await execFile('git', ['checkout', '-']);
    }
    try {
        if (gitStashApplied)
            await execFile('git', ['stash', 'pop']);
    }
    catch { }
}
exports.checkoutRefAndExecute = checkoutRefAndExecute;
async function getGitSha() {
    if (process.env.NODE_ENV === 'test') {
        return 'test';
    }
    const { stdout } = await execFile('git', ['rev-parse', '--verify', 'HEAD']);
    return stdout.trim();
}
exports.getGitSha = getGitSha;
