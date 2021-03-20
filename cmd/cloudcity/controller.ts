import { Controller } from 'stimulus';

// Reference: https://stimulus.hotwire.dev/reference/controllers

export default class extends Controller {
  // Reference: https://stimulus.hotwire.dev/reference/targets
  static targets = [];
  // Example property:
  // readonly widgetTarget!: HTMLElement;

  // Reference: https://stimulus.hotwire.dev/reference/values
  static values = {};
  // Example property:
  // textValue!: string;

  // Reference: https://stimulus.hotwire.dev/reference/css-classes
  static classes = ['loading'];
  // Example property:
  // readonly loadingClass!: string;

  initialize() {
    // Invoked once when the controller is first instantiated
  }

  connect() {
    // Invoked anytime the controller is connected to the DOM
    // Reference: https://stimulus.hotwire.dev/reference/lifecycle-callbacks#connection
  }

  disconnect() {
    // Invoked anytime the controller is disconnected from the DOM
    // Reference: https://stimulus.hotwire.dev/reference/lifecycle-callbacks#disconnection
  }

  // Example action:
  // next(event: Event) {
  // }
  //
  // Reference: https://stimulus.hotwire.dev/reference/actions
}
