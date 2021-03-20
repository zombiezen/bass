import { Application } from 'stimulus';
import { definitions } from 'stimulus:./controllers';
import '@hotwired/turbo';

const application = Application.start();
application.load(definitions);
