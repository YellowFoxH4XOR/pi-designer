import { render } from 'preact';
import { App } from './App';
import { ModelProvider } from './model-context';
import './styles.css';
import './models.css';

render(<ModelProvider><App /></ModelProvider>, document.getElementById('app')!);
