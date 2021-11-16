import { getOwner } from '@ember/application';
import { computed } from '@ember/object';
import Service, { inject as service } from '@ember/service';

export default class BreadcrumbsService extends Service {
  @service router;

  generatePathHierarchy = (routeName = '') =>
    routeName
      .split('.')
      .without('')
      .map((_, index, allSegments) => allSegments.slice(0, index + 1).join('.'));

  generateCrumb = route => {
    if (route.breadcrumbs) {
      const areBreadcrumbsAFunction = typeof route.breadcrumbs === 'function';

      if (areBreadcrumbsAFunction) {
        return route.breadcrumbs(route.get('controller.model')) ?? [];
      } else {
        return route.breadcrumbs;
      }
    } else {
      return [];
    }
  };

  generateBreadcrumbs = (fullRouteNamesOfPathHierarchy, owner) =>
    fullRouteNamesOfPathHierarchy.reduce((crumbs, routeName) => {
      const route = owner.lookup(`route:${routeName}`);

      // Routes can reset the breadcrumb trail to start anew even
      // if the route is deeply nested.
      if (route.resetBreadcrumbs) {
        crumbs = [];
      }

      // Breadcrumbs are either an array of static crumbs
      // or a function that returns breadcrumbs given the current
      // model for the route's controller.
      let crumb = this.generateCrumb(route);
      crumbs.push(...crumb);

      return crumbs;
    }, []);

  // currentURL is only used to listen to all transitions.
  // currentRouteName has all information necessary to compute breadcrumbs,
  // but it doesn't change when a transition to the same route with a different
  // model occurs.
  @computed('router.{currentURL,currentRouteName}')
  get breadcrumbs() {
    const owner = getOwner(this);
    const allRoutes = this.generatePathHierarchy(this.router.currentRouteName);
    return this.generateBreadcrumbs(allRoutes, owner);
  }
}
