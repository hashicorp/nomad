STORYBOOK_LINK=true ember build
mkdir -p ui-dist/ui
mv dist/* ui-dist/ui/

yarn build-storybook
yarn extract-storybook
cp storybook-static/stories.json ui-dist/ui/
mv storybook-static ui-dist/storybook/

