mkdir -p ui-dist/ui
mv ui/dist/* ui-dist/ui

npm install -g netlify-cli

netlify deploy --dir=ui-dist > netlify.log
cat netlify.log

NETLIFY_DEPLOYMENT_URL=$(echo netlify.log | awk '{ print $NF }')

curl -X POST \
    --data "{\"state\": \"success\", \"target_url\": \"$NETLIFY_DEPLOYMENT_URL\", \"description\": \"Visit a deployment for this PR\", \"context\": \"deployments\"}" \
    -H "Authorization: token $GITHUB_STATUS_TOKEN" \
    https://api.github.com/repos/hashicorp/nomad/statuses/$TRAVIS_PULL_REQUEST_SHA
