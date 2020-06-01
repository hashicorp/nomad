import Component from '@ember/component';
import { computed } from '@ember/object';
import { filterBy } from '@ember/object/computed';

export default Component.extend({
  tagName: '',

  model: null,

  allocation: computed('model', function() {
    if (this.model.allocation) {
      return this.model.allocation;
    } else {
      return this.model;
    }
  }),

  taskState: computed('model', function() {
    if (this.model.allocation) {
      return this.model;
    }
  }),

  type: computed('taskState', function() {
    if (this.taskState) {
      return 'task';
    } else {
      return 'allocation';
    }
  }),

  directories: filterBy('directoryEntries', 'IsDir'),
  files: filterBy('directoryEntries', 'IsDir', false),

  sortedDirectoryEntries: computed(
    'directoryEntries.[]',
    'sortProperty',
    'sortDescending',
    function() {
      const sortProperty = this.sortProperty;

      const directorySortProperty = sortProperty === 'Size' ? 'Name' : sortProperty;

      const sortedDirectories = this.directories.sortBy(directorySortProperty);
      const sortedFiles = this.files.sortBy(sortProperty);

      const sortedDirectoryEntries = sortedDirectories.concat(sortedFiles);

      if (this.sortDescending) {
        return sortedDirectoryEntries.reverse();
      } else {
        return sortedDirectoryEntries;
      }
    }
  ),

});
