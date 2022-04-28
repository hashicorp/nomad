import Route from '@ember/routing/route';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';

export default class ServerRoute extends Route.extend(WithModelErrorHandling) {
  breadcrumbs(model) {
    if (!model) return [];
    return [
      {
        label: model.name,
        args: ['servers.server', model.id],
      },
    ];
  }
}
