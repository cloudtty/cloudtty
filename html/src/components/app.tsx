import { h, Component } from 'preact';

import { ITerminalOptions, ITheme } from 'xterm';
import { ClientOptions, Xterm } from './terminal';

if ((module as any).hot) {
    // tslint:disable-next-line:no-var-requires
    require('preact/debug');
}

const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
const path = window.location.pathname.replace(/[\/]+$/, '');
const wsUrl = [protocol, '//', window.location.host, path, '/ws', window.location.search].join('');
const tokenUrl = [window.location.protocol, '//', window.location.host, path, '/token'].join('');
const clientOptions = {
    rendererType: 'webgl',
    disableLeaveAlert: false,
    disableResizeOverlay: false,
    titleFixed: null,
} as ClientOptions;
const termOptions = {
    fontSize: 13,
    fontFamily: 'Menlo For Powerline,Consolas,Liberation Mono,Menlo,Courier,monospace',
    macOptionClickForcesSelection: true,
    macOptionIsMeta: true,
    theme: {
        foreground: '#d2d2d2',
        background: '#2b2b2b',
        cursor: '#adadad',
        black: '#000000',
        red: '#d81e00',
        green: '#5ea702',
        yellow: '#cfae00',
        blue: '#427ab3',
        magenta: '#89658e',
        cyan: '#00a7aa',
        white: '#dbded8',
        brightBlack: '#686a66',
        brightRed: '#f54235',
        brightGreen: '#99e343',
        brightYellow: '#fdeb61',
        brightBlue: '#84b0d8',
        brightMagenta: '#bc94b7',
        brightCyan: '#37e6e8',
        brightWhite: '#f1f1f0',
    } as ITheme,
} as ITerminalOptions;

interface State {
    showRz: boolean;
    showSz: boolean;
}

const state = { showRz: false, showSz: false }

export class App extends Component {
    hideSz = (v: boolean) => {
        this.setState({ showSz: v });
    }
    hideRz = (v: boolean) => {
        this.setState({ showRz: v });
    }
    render(_, { showRz, showSz }: State) {
        return (
            <div style="width: 100%; height: 100%">
                <div className="terminal-operator">
                    <button className="terminal-operator__btn" onClick={() => this.hideRz(true)}>Upload</button>
                    <button className="terminal-operator__btn" onClick={() => this.hideSz(true)}>Download</button>
                </div>
                <Xterm
                    id="terminal-container"
                    wsUrl={wsUrl}
                    tokenUrl={tokenUrl}
                    clientOptions={clientOptions}
                    termOptions={termOptions}
                    showRz={showRz}
                    showSz={showSz}
                    hideDownload={this.hideSz}
                    hideUpload={this.hideRz}
                />
            </div>
        );
    }
}