import * as vscode from 'vscode';
import * as path from 'path';
import * as os from 'os';
import * as fs from 'fs';

export function activate(context: vscode.ExtensionContext) {
	let disposable = vscode.commands.registerCommand('codeface.setup', async (gitUrl?: string, parentDir?: string) => {
		vscode.commands.executeCommand("git.clone", gitUrl, parentDir);
		// TODO: download Go
	});
	context.subscriptions.push(disposable);

	// Trigger a setup after activate
	let gitUrl = process.env.GIT_REPO;
	if (gitUrl) {
		let repoName = decodeURI(gitUrl).replace(/[\/]+$/, '').replace(/^.*[\/\\]/, '').replace(/\.git$/, '') || 'repository';
		let parentDir = path.join(os.homedir(), "project");
		// Don't git clone if a folder already exists
		if (!fs.existsSync(path.join(parentDir, repoName))) {
			vscode.commands.executeCommand("codeface.setup", gitUrl, parentDir);
		}
	}
}

// this method is called when your extension is deactivated
export function deactivate() { }
