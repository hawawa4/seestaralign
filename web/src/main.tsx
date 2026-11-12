import { render } from 'preact';
// Base styles first: component stylesheets (imported by App) override them.
import './styles/tokens.css';
import './styles/base.css';
import './styles/dim.css';
import { App } from './app';

render(<App />, document.getElementById('app')!);
