import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { filterBy } from '@ember/object/computed';
import { isEmpty } from '@ember/utils';

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

  breadcrumbs: computed('path', function() {
    const breadcrumbs = this.path
      .split('/')
      .reject(isEmpty)
      .reduce((breadcrumbs, pathSegment, index) => {
        let breadcrumbPath;

        if (index > 0) {
          const lastBreadcrumb = breadcrumbs[index - 1];
          breadcrumbPath = `${lastBreadcrumb.path}/${pathSegment}`;
        } else {
          breadcrumbPath = pathSegment;
        }

        breadcrumbs.push({
          name: pathSegment,
          path: breadcrumbPath,
        });

        return breadcrumbs;
      }, []);

    if (breadcrumbs.length) {
      breadcrumbs[breadcrumbs.length - 1].isLast = true;
    }

    return breadcrumbs;
  }),
});
