const path = require("path");
const { execSync } = require("child_process");
const core = require("../../../website/node_modules/@actions/core");
const github = require("../../../website/node_modules/@actions/github");

const { GITHUB_TOKEN, GITHUB_SHA } = process.env;

const CHECK_NAME = "[Check] Broken links";

const octokit = new github.GitHub(GITHUB_TOKEN);

async function createCheck() {
  const { data } = await octokit.checks.create({
    ...github.context.repo,
    name: CHECK_NAME,
    head_sha: GITHUB_SHA,
    status: "in_progress",
    started_at: new Date()
  });

  return data.id;
}

async function updateCheck(id, checkResults) {
  await octokit.checks.update({
    ...github.context.repo,
    ...checkResults,
    name: CHECK_NAME,
    check_run_id: id
  });
}

async function run() {
  const id = await createCheck();
  const root = path.join(__dirname, "../../..");
  // const deployUrl = core.getInput("baseUrl", { required: true });
  const deployUrl = "https://nomadproject.io";

  try {
    // Run the link check against the PR preview link
    // const output = execSync(
    //   `../../../website/node_modules/.bin/linkcheck ${deployUrl}`
    // );
    const output = execSync(`ls ../../../website/node_modules/.bin`);

    // WIP
    console.log(output);
    await updateCheck(id, { output: { summary: output } });
  } catch (error) {
    core.setFailed(`Action failed with message: ${error.message}`);
  }
}

run().catch(error => core.setFailed(error.message));
