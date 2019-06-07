npm install -g netlify-cli

cd ui
./node_modules/bin/ember build -prod
cd ..

echo "Listing ui/"
ls ui

mkdir -p ui-dist/ui
mv ui/dist/* ui-dist/ui

netlify deploy --dir=ui-dist > netlify.log
cat netlify.log

NETLIFY_DEPLOYMENT_URL=$(cat netlify.log | awk '{ print $NF }')

echo "Netlify deployment URL: ${NETLIFY_DEPLOYMENT_URL}"

curl -X POST \
    --data "{\"state\": \"success\", \"target_url\": \"$NETLIFY_DEPLOYMENT_URL\", \"description\": \"Visit a deployment for this PR\", \"context\": \"deployments\"}" \
    -H "Authorization: token $GITHUB_STATUS_TOKEN" \
    https://api.github.com/repos/hashicorp/nomad/statuses/$TRAVIS_COMMIT
