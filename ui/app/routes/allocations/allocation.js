import Route from '@ember/routing/route';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';

export default Route.extend(WithModelErrorHandling);
// import notifyError from 'nomad-ui/utils/notify-error';
// import RSVP from 'rsvp';

// export default Route.extend(
//   {
//     model(params, transition) {
//       return this.get('store')
//         .findRecord('allocation', params.allocation_id, { reload: true })
//         .then(allocation => {
//           return RSVP.all([allocation.get('job')]).then((job) => {
//             // why is job an array? Can allocations have multiple jobs?
//             return allocation;
//           });
//         })
//         .catch(notifyError(this));
//     },
//   }
// );
