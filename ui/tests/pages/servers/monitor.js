import {
  create,
  attribute,
  clickable,
  collection,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';
import { run } from '@ember/runloop';
import { selectOpen, selectOpenChoose } from '../../utils/ember-power-select-extensions';

export default create({
  visit: visitable('/servers/:name/monitor'),

  breadcrumbs: collection('[data-test-breadcrumb]', {
    id: attribute('data-test-breadcrumb'),
    text: text(),
    visit: clickable(),
  }),

  breadcrumbFor(id) {
    return this.breadcrumbs.toArray().find(crumb => crumb.id === id);
  },

  logsArePresent: isPresent('[data-test-log-box]'),

  error: {
    isShown: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },

  async selectLogLevel(level) {
    const contentId = await selectOpen('[data-test-level-switcher]');
    run.later(run, run.cancelTimers, 500);
    await selectOpenChoose(contentId, level);
  },
});
