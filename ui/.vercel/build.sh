ember build
mkdir -p ui-dist/ui
mv dist/* ui-dist/ui/

yarn build-storybook
mv storybook-static ui-dist/storybook/

cp .vercel/index.html ui-dist/index.html
