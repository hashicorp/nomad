import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { filterBy } from '@ember/object/computed';

export default Controller.extend({
  directories: filterBy('ls', 'IsDir'),
  files: filterBy('ls', 'IsDir', false),

  pathComponents: computed('pathWithTaskName', function() {
    return this.pathWithTaskName.split('/').reject(s => s === '');
  }),
});
