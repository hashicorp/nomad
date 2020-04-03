import Service from '@ember/service';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

export default Service.extend({
  pageSize: localStorageProperty('nomadPageSize', 25),
});
