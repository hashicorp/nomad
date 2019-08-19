import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { filterBy } from '@ember/object/computed';

export default Controller.extend({
  queryParams: {
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  sortProperty: 'Name',
  sortDescending: false,

  path: null,
  task: null,
  directoryEntries: null,
  isFile: null,
  stat: null,

  directories: filterBy('directoryEntries', 'IsDir'),
  files: filterBy('directoryEntries', 'IsDir', false),

  pathWithLeadingSlash: computed('path', function() {
    const path = this.path;

    if (path.startsWith('/')) {
      return path;
    } else {
      return `/${path}`;
    }
  }),

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
