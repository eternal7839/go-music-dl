//go:build android

package main

import (
	"log"

	"gioui.org/app"
	"git.wow.st/gmp/jni"
)

func (a *desktopApp) configureBundledFFmpeg(evt app.ViewEvent) {
	if a.bundledFFmpegOnce {
		return
	}
	androidEvt, ok := evt.(app.AndroidViewEvent)
	if !ok || androidEvt.View == 0 {
		return
	}

	a.bundledFFmpegOnce = true
	view := jni.Object(androidEvt.View)
	go a.window.Run(func() {
		if err := configureBundledFFmpegFromView(view); err != nil {
			log.Printf("configure bundled ffmpeg: %v", err)
		}
	})
}

func configureBundledFFmpegFromView(view jni.Object) error {
	return jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		activity, err := activityFromView(env, view)
		if err != nil {
			return err
		}

		nativeLibraryDir, err := nativeLibraryDirFromContext(env, activity)
		if err != nil {
			return err
		}
		return configureBundledFFmpegFromNativeLibraryDir(nativeLibraryDir)
	})
}

func nativeLibraryDirFromContext(env jni.Env, context jni.Object) (string, error) {
	contextClass := jni.GetObjectClass(env, context)
	applicationInfo, err := jni.CallObjectMethod(
		env,
		context,
		jni.GetMethodID(env, contextClass, "getApplicationInfo", "()Landroid/content/pm/ApplicationInfo;"),
	)
	if err != nil {
		return "", err
	}

	applicationInfoClass := jni.GetObjectClass(env, applicationInfo)
	nativeLibraryDir := jni.GetObjectField(
		env,
		applicationInfo,
		jni.GetFieldID(env, applicationInfoClass, "nativeLibraryDir", "Ljava/lang/String;"),
	)
	return jni.GoString(env, jni.String(nativeLibraryDir)), nil
}
