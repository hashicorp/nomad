import { configure } from '@storybook/ember';

configure(require.context('../stories', true, /\.stories\.js$/), module);
