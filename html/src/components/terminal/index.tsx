import { bind } from 'decko';
import { Component, h } from 'preact';
import { ITerminalOptions, RendererType, Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { WebglAddon } from 'xterm-addon-webgl';
import { WebLinksAddon } from 'xterm-addon-web-links';

import { OverlayAddon } from './overlay';
import { ZmodemAddon, FlowControl } from '../zmodem';

import 'xterm/css/xterm.css';

interface TtydTerminal extends Terminal {
    fit(): void;
}

declare global {
    interface Window {
        term: TtydTerminal;
    }
}

const enum Command {
    // server side
    OUTPUT = '0',
    SET_WINDOW_TITLE = '1',
    SET_PREFERENCES = '2',

    // client side
    INPUT = '0',
    RESIZE_TERMINAL = '1',
    PAUSE = '2',
    RESUME = '3',
}

export interface ClientOptions {
    rendererType: 'dom' | 'canvas' | 'webgl';
    disableLeaveAlert: boolean;
    disableResizeOverlay: boolean;
    titleFixed: string;
}

interface Props {
    id: string;
    wsUrl: string;
    tokenUrl: string;
    clientOptions: ClientOptions;
    termOptions: ITerminalOptions;
    showRz: boolean;
    showSz: boolean;
    hideDownload: (v: boolean) => void;
    hideUpload: (v: boolean) => void;
}

export class Xterm extends Component<Props> {
    private textEncoder: TextEncoder;
    private textDecoder: TextDecoder;
    private container: HTMLElement;
    private terminal: Terminal;

    private fitAddon: FitAddon;
    private overlayAddon: OverlayAddon;
    private zmodemAddon: ZmodemAddon;
    private webglAddon: WebglAddon;

    private socket: WebSocket;
    private token: string;
    private opened = false;
    private title: string;
    private titleFixed: string;
    private resizeTimeout: number;
    private resizeOverlay = true;
    private reconnect = true;
    private doReconnect = true;

    private startTimestamp: number;
    private endTimestamp: number;
    private ttl: number;

    constructor(props: Props) {
        super(props);

        this.textEncoder = new TextEncoder();
        this.textDecoder = new TextDecoder();
        this.fitAddon = new FitAddon();
        this.overlayAddon = new OverlayAddon();
    }

    async componentDidMount() {
        await this.refreshToken();
        this.openTerminal();
        this.connect();

        window.addEventListener('resize', this.onWindowResize);
        window.addEventListener('beforeunload', this.onWindowUnload);
    }

    componentDidUpdate() {
        if (this.props.showRz) {
            const { socket, textEncoder } = this;
            socket.send(textEncoder.encode(Command.INPUT + 'r'));
            socket.send(textEncoder.encode(Command.INPUT + 'z'));
            socket.send(textEncoder.encode(Command.INPUT + '\n'));
        }
    }

    componentWillUnmount() {
        this.socket.close();
        this.terminal.dispose();

        window.removeEventListener('resize', this.onWindowResize);
        window.removeEventListener('beforeunload', this.onWindowUnload);
    }

    render({ id }: Props) {
        const control = {
            limit: 100000,
            highWater: 10,
            lowWater: 4,
            pause: () => this.pause(),
            resume: () => this.resume(),
        } as FlowControl;

        return (
            <div id={id} ref={c => (this.container = c)}>
                <ZmodemAddon
                    ref={c => (this.zmodemAddon = c)}
                    sender={this.sendData}
                    control={control}
                    modalDownload={this.props.showSz}
                    modalUpload={this.props.showRz}
                    change={(v) => { this.onChange((v)) }}
                    hideDownload={(v) => { this.onHideDownload((v)) }}
                    hideUpload={(v) => { this.onHideUpload((v)) }}
                />
            </div>
        );
    }

    onChange(v) {
        if (!v) return;
        const { socket, textEncoder } = this;
        socket.send(textEncoder.encode(Command.INPUT + 's'));
        socket.send(textEncoder.encode(Command.INPUT + 'z'));
        socket.send(textEncoder.encode(Command.INPUT + ' '));
        for (let i in v) {
            socket.send(textEncoder.encode(Command.INPUT + v[i]));
        }
        socket.send(textEncoder.encode(Command.INPUT + '\n'));
    }

    onHideDownload(v) {
        this.props.hideDownload(v);
    }

    onHideUpload(v) {
        this.props.hideUpload(v);
    }

    @bind
    private pause() {
        const { textEncoder, socket } = this;
        socket.send(textEncoder.encode(Command.PAUSE));
    }

    @bind
    private resume() {
        const { textEncoder, socket } = this;
        socket.send(textEncoder.encode(Command.RESUME));
    }

    @bind
    private sendData(data: ArrayLike<number>) {
        const { socket } = this;
        const payload = new Uint8Array(data.length + 1);
        payload[0] = Command.INPUT.charCodeAt(0);
        payload.set(data, 1);
        socket.send(payload);
    }

    @bind
    private async refreshToken() {
        try {
            const resp = await fetch(this.props.tokenUrl);
            if (resp.ok) {
                const json = await resp.json();
                this.token = json.token;
            }
        } catch (e) {
            console.error(`[ttyd] fetch ${this.props.tokenUrl}: `, e);
        }
    }

    @bind
    private onWindowResize() {
        const { fitAddon } = this;
        clearTimeout(this.resizeTimeout);
        this.resizeTimeout = setTimeout(() => fitAddon.fit(), 250) as any;
    }

    @bind
    private onWindowUnload(event: BeforeUnloadEvent): any {
        const { socket } = this;
        if (socket && socket.readyState === WebSocket.OPEN) {
            const message = 'Close terminal? this will also terminate the command.';
            event.returnValue = message;
            return message;
        }
        event.preventDefault();
    }

    @bind
    private openTerminal() {
        this.terminal = new Terminal(this.props.termOptions);
        const { terminal, container, fitAddon, overlayAddon } = this;
        window.term = terminal as TtydTerminal;
        window.term.fit = () => {
            this.fitAddon.fit();
        };

        terminal.loadAddon(fitAddon);
        terminal.loadAddon(overlayAddon);
        terminal.loadAddon(new WebLinksAddon());
        terminal.loadAddon(this.zmodemAddon);

        terminal.onTitleChange(data => {
            if (data && data !== '' && !this.titleFixed) {
                document.title = data + ' | ' + this.title;
            }
        });
        terminal.onData(this.onTerminalData);
        terminal.onResize(this.onTerminalResize);
        if (document.queryCommandSupported && document.queryCommandSupported('copy')) {
            terminal.onSelectionChange(() => {
                if (terminal.getSelection() === '') return;
                overlayAddon.showOverlay('\u2702', 200);
                document.execCommand('copy');
            });
        }
        terminal.open(container);
        fitAddon.fit();
    }

    @bind
    private connect() {
        if (!this.startTimestamp) {
            this.startTimestamp = new Date().getTime();
        }

        this.socket = new WebSocket(this.props.wsUrl, ['tty']);

        const { socket } = this;

        socket.binaryType = 'arraybuffer';
        socket.onopen = this.onSocketOpen;
        socket.onmessage = this.onSocketData;
        socket.onclose = this.onSocketClose;
        socket.onerror = this.onSocketError;

        if (!this.ttl) {
            let params = new URL(location.href).searchParams;
            const ttlParams = params.get('ttl');

            if (ttlParams) {
                let num = Number(ttlParams);

                this.ttl = (!isNaN(num) && ttlParams !== "") ? num : Infinity;
            } else {
                this.ttl = Infinity;
            }
        }
    }

    @bind
    private setRendererType(value: 'webgl' | RendererType) {
        const { terminal } = this;

        const disposeWebglRenderer = () => {
            try {
                this.webglAddon?.dispose();
            } catch {
                // ignore
            }
            this.webglAddon = undefined;
        };

        switch (value) {
            case 'webgl':
                if (this.webglAddon) return;
                try {
                    if (window.WebGL2RenderingContext && document.createElement('canvas').getContext('webgl2')) {
                        this.webglAddon = new WebglAddon();
                        this.webglAddon.onContextLoss(() => {
                            disposeWebglRenderer();
                        });
                        terminal.loadAddon(this.webglAddon);
                        console.log(`[ttyd] WebGL renderer enabled`);
                    }
                } catch (e) {
                    console.warn(`[ttyd] webgl2 init error`, e);
                }
                break;
            default:
                disposeWebglRenderer();
                console.log(`[ttyd] option: rendererType=${value}`);
                terminal.options.rendererType = value;
                break;
        }
    }

    @bind
    private applyOptions(options: any) {
        const { terminal, fitAddon } = this;

        Object.keys(options).forEach(key => {
            const value = options[key];
            switch (key) {
                case 'rendererType':
                    this.setRendererType(value);
                    break;
                case 'disableLeaveAlert':
                    if (value) {
                        window.removeEventListener('beforeunload', this.onWindowUnload);
                        console.log('[ttyd] Leave site alert disabled');
                    }
                    break;
                case 'disableResizeOverlay':
                    if (value) {
                        console.log(`[ttyd] Resize overlay disabled`);
                        this.resizeOverlay = false;
                    }
                    break;
                case 'disableReconnect':
                    if (value) {
                        console.log(`[ttyd] Reconnect disabled`);
                        this.reconnect = false;
                    }
                    break;
                case 'titleFixed':
                    if (!value || value === '') return;
                    console.log(`[ttyd] setting fixed title: ${value}`);
                    this.titleFixed = value;
                    document.title = value;
                    break;
                default:
                    console.log(`[ttyd] option: ${key}=${JSON.stringify(value)}`);
                    if (terminal.options[key] instanceof Object) {
                        terminal.options[key] = Object.assign({}, terminal.options[key], value);
                    } else {
                        terminal.options[key] = value;
                    }
                    if (key.indexOf('font') === 0) fitAddon.fit();
                    break;
            }
        });
    }

    @bind
    private onSocketOpen() {
        console.log('[ttyd] websocket connection opened');

        const { socket, textEncoder, terminal, fitAddon, overlayAddon } = this;
        const dims = fitAddon.proposeDimensions();

        const authTokenMsg = JSON.stringify({ AuthToken: this.token });
        socket.send(textEncoder.encode(authTokenMsg));

        const resizeMsg = JSON.stringify({ columns: dims.cols, rows: dims.rows });
        socket.send(textEncoder.encode(Command.RESIZE_TERMINAL + resizeMsg));

        const uploadFileToPod = (fileUrl) => {
            const { socket, textEncoder } = this;
            const command = 'cd /tmp/ && rz && ls -t | head -n 1 | xargs -i{}  kubectl cp -c "${CONTAINER}" {} ${POD_NAMESPACE}/${POD_NAME}:' + `${fileUrl}`;
            socket.send(textEncoder.encode(Command.INPUT + command));
            socket.send(textEncoder.encode(Command.INPUT + '\n'));

            const reminder = '\n the file have uploaded...';
            socket.send(textEncoder.encode(Command.INPUT + reminder));
        }

        const downLoadFileToPod = (fileUrl) => {
            const { socket, textEncoder } = this;
            const filePath = fileUrl.split('/');
            const fileName = filePath[filePath.length - 1];
            const fileUrlStartWithSlash = `/${fileUrl}`.replace(/^[\/]+/, '/');

            console.log(`[ttyd] download fileName : ${fileName}`);

            const command = 'kubectl cp -c "${CONTAINER}" ${POD_NAMESPACE}/${POD_NAME}:' + `${fileUrlStartWithSlash}` + ` /tmp/${fileName}` + ` && sz /tmp/${fileName}`;
            socket.send(textEncoder.encode(Command.INPUT + command));
            socket.send(textEncoder.encode(Command.INPUT + '\n'));

            const reminder = '\n the file have downloaded...';
            socket.send(textEncoder.encode(Command.INPUT + reminder));
        }

        const path = window.location.pathname.replace(/[\/]+$/, '');

        const uploadPathArr = path.split("/upload")
        if (uploadPathArr.length > 1) {
            var fileUrl = uploadPathArr[uploadPathArr.length - 1]
            if (fileUrl.length == 0) {
                fileUrl = "/"
            }
            console.log(`[ttyd] upload fileUrl : ${fileUrl}`);
            uploadFileToPod(fileUrl)
        }

        const downloadPathArr = path.split("/download/")
        if (downloadPathArr.length > 1) {
            const fileUrl = downloadPathArr[downloadPathArr.length - 1]
            if (fileUrl.length > 0) {
                console.log(`[ttyd] download fileUrl : ${fileUrl}`);
                downLoadFileToPod(decodeURIComponent(fileUrl));
            }
        }

        if (this.opened) {
            terminal.reset();
            terminal.resize(dims.cols, dims.rows);
            overlayAddon.showOverlay('Reconnected', 300);
        } else {
            this.opened = true;
        }

        this.doReconnect = this.reconnect;

        terminal.focus();
    }

    @bind
    private onSocketClose(event: CloseEvent) {
        console.log(`[ttyd] websocket connection closed with code: ${event.code}`);

        if (this.ttl !== Infinity) {
            this.endTimestamp = new Date().getTime();
            const durationTime = Math.floor((this.endTimestamp - this.startTimestamp) / 1000);

            if ((durationTime + 10) >= this.ttl) {
                const { overlayAddon } = this;
                overlayAddon.showOverlay('The terminal has ended.', null);
                return;
            }

        }

        const { refreshToken, connect, doReconnect, overlayAddon } = this;
        overlayAddon.showOverlay('Connection Closed', null);

        // 1000: CLOSE_NORMAL
        if (event.code !== 1000 && doReconnect) {
            const { terminal } = this;
            const keyDispose = terminal.onKey(e => {
                const event = e.domEvent;
                if (event.key === 'Enter') {
                    keyDispose.dispose();
                    overlayAddon.showOverlay('Reconnecting...', null);
                    refreshToken().then(connect);
                }
            });
            overlayAddon.showOverlay('Press ⏎ to Reconnect', null);
        }
    }

    @bind
    private onSocketError(event: Event) {
        console.error('[ttyd] websocket connection error: ', event);
        this.doReconnect = false;
    }

    @bind
    private onSocketData(event: MessageEvent) {
        const { textDecoder, zmodemAddon } = this;
        const rawData = event.data as ArrayBuffer;
        const cmd = String.fromCharCode(new Uint8Array(rawData)[0]);
        const data = rawData.slice(1);

        switch (cmd) {
            case Command.OUTPUT:
                zmodemAddon.consume(data);
                break;
            case Command.SET_WINDOW_TITLE:
                this.title = textDecoder.decode(data);
                document.title = this.title;
                break;
            case Command.SET_PREFERENCES:
                const prefs = JSON.parse(textDecoder.decode(data));
                this.applyOptions(Object.assign({}, this.props.clientOptions, prefs));
                break;
            default:
                console.warn(`[ttyd] unknown command: ${cmd}`);
                break;
        }
    }

    @bind
    private onTerminalResize(size: { cols: number; rows: number }) {
        const { overlayAddon, socket, textEncoder, resizeOverlay } = this;
        if (socket && socket.readyState === WebSocket.OPEN) {
            const msg = JSON.stringify({ columns: size.cols, rows: size.rows });
            socket.send(textEncoder.encode(Command.RESIZE_TERMINAL + msg));
        }
        if (resizeOverlay) {
            setTimeout(() => {
                overlayAddon.showOverlay(`${size.cols}x${size.rows}`);
            }, 500);
        }
    }

    @bind
    private onTerminalData(data: string) {
        const { socket, textEncoder } = this;
        if (socket.readyState === WebSocket.OPEN) {
            socket.send(textEncoder.encode(Command.INPUT + data));
        }
    }
}
