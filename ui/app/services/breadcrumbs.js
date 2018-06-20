import { getOwner } from '@ember/application';
import Service, { inject as service } from '@ember/service';
import { computed } from '@ember/object';

export default Service.extend({
  router: service(),

  breadcrumbs: computed('router.currentRouteName', function() {
    const owner = getOwner(this);
    const allRoutes = this.get('router.currentRouteName')
      .split('.')
      .map((segment, index, allSegments) => allSegments.slice(0, index + 1).join('.'));

    let crumbs = [];
    allRoutes.forEach(routeName => {
      const route = owner.lookup(`route:${routeName}`);

      // Routes can reset the breadcrumb trail to start anew even
      // if the route is deeply nested.
      if (route.get('resetBreadcrumbs')) {
        crumbs = [];
      }

      // Breadcrumbs are either an array of static crumbs
      // or a function that returns breadcrumbs given the current
      // model for the route's controller.
      let breadcrumbs = route.get('breadcrumbs') || [];
      if (typeof breadcrumbs === 'function') {
        breadcrumbs = breadcrumbs(route.get('controller.model'));
      }

      crumbs.push(...breadcrumbs);
    });

    return crumbs;
  }),
});
