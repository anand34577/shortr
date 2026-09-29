package io.github.anand34577.shortr

import android.app.Application
import android.content.Context
import io.github.anand34577.shortr.data.AppContainer

class ShortrApp : Application() {
    lateinit var container: AppContainer
        private set

    override fun onCreate() {
        super.onCreate()
        container = AppContainer(this)
    }
}

val Context.container: AppContainer get() = (applicationContext as ShortrApp).container
