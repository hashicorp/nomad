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
  const deployUrl = core.getInput("baseUrl", { required: true });

  console.log(`checking for links on ${deployUrl}`);

  try {
    // Run the link check against the PR preview link
    let conclusion = "success";
    let output;
    try {
      output = String(
        execSync(
          `./website/node_modules/dart-linkcheck/bin/linkcheck-linux ${deployUrl}`
        )
      );
    } catch (err) {
      // the command fails if any links are broken, but we still want to log the output
      conclusion = "failure";
      output = String(err.stdout);
    }

    await updateCheck(id, {
      output: Object.assign(
        {},
        { conclusion },
        {
          title: "test",
          summary:
            conclusion === "failure"
              ? "Broken internal links found"
              : "All interal links are working!",
          text: String(output)
        }
      )
    });
  } catch (error) {
    console.log(error);
    core.setFailed(`Action failed with message: ${error.message}`);
  }
}

run().catch(error => {
  console.log(error);
  // core.setFailed(error.message)
});
