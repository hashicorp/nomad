import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { filterBy } from '@ember/object/computed';
import { isEmpty } from '@ember/utils';

export default Controller.extend({
  directories: filterBy('directoryEntries', 'IsDir'),
  files: filterBy('directoryEntries', 'IsDir', false),

  breadcrumbs: computed('path', 'model.name', function() {
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
