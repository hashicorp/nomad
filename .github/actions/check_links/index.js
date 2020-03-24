const { execSync } = require("child_process");
const core = require("../../../website/node_modules/@actions/core");
const github = require("../../../website/node_modules/@actions/github");

const { GITHUB_TOKEN } = process.env;

const CHECK_NAME = "Check Broken Links";

const octokit = new github.GitHub(GITHUB_TOKEN);

async function createCheck() {
  const { data } = await octokit.checks.create({
    ...github.context.repo,
    name: CHECK_NAME,
    head_sha: github.context.payload.pull_request.head.sha,
    status: "in_progress",
    started_at: new Date()
  });

  console.log(data);

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
  let output;

  console.log(`check created for ${id}`);
  console.log(`checking for links on ${deployUrl}`);

  try {
    // Run the link check against the PR preview link
    let conclusion = "success";
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
      conclusion,
      status: "completed",
      output: Object.assign(
        {},
        {
          title: "Broken Links Check",
          summary:
            conclusion === "failure"
              ? "🚫 **Broken internal links found**"
              : "✅ **All interal links are working!**",
          text: String(output)
        }
      )
    });
  } catch (error) {
    console.log(error);
    return core.setFailed(`Action failed with message: ${error.message}`);
  }

  console.log(output);
}

run().catch(error => {
  console.log(error);
  core.setFailed(error.message);
});
