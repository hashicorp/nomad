import Component from '@ember/component';
import { computed } from '@ember/object';
import { isEmpty } from '@ember/utils';

export default Component.extend({
  tagName: 'nav',
  classNames: ['breadcrumb'],

  'data-test-fs-breadcrumbs': true,

  task: null,
  path: null,

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
