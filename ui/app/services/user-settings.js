import Service from '@ember/service';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

export default class UserSettingsService extends Service {
  @localStorageProperty('nomadPageSize', 25) pageSize;
  @localStorageProperty('nomadLogMode', 'stdout') logMode;
  @localStorageProperty('nomadTopoVizPollingNotice', true) showTopoVizPollingNotice;
}
