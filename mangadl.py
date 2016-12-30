from bs4 import BeautifulSoup
import gevent
from gevent.queue import Queue
from gevent import monkey
import requests
from time import time
from pprint import pprint
from PIL import Image
from StringIO import StringIO

monkey.patch_socket()

sites = {
    'mangareader': {
        'url': 'http://www.mangareader.net',
        'page_list': lambda p: ['http://www.mangareader.net' + x['value'] for x in p.find_all('option')],
        'img': lambda p: p.find(id='img').get('src')
    }
}

class Gevent_download:
    def __init__(self, tasks, workers=4, filename='out'):
        self.queue = Queue()
        self.tasks = tasks  # format is [(pagenum, pageurl)]
        self.workers = workers  # num of workers
        self.out = []  # list of results
        self.filename = filename.replace('/', '')

    def worker(self,n, func):
        while not self.queue.empty():
            start_time = time()
            task = self.queue.get()

            # get page
            res_raw = requests.get(task[1])
            res = BeautifulSoup(res_raw.text, 'html.parser')
            img = sites['mangareader']['img'](res)

            # get img
            jpg_raw = requests.get(img)
            jpg = Image.open(StringIO(jpg_raw.content))

            # finish task
            self.out.append((task[0], img, jpg))
            print 'Worker {0} finished {1} in {2}'.format(n, task, time()-start_time)

    def execute(self):
        for task in self.tasks:
            self.queue.put_nowait(task)
        gevent.joinall([gevent.spawn(self.worker, x) for x in xrange(self.workers)])
        self.out.sort()
        return self.out

    def save_jpg(self):
        for img in self.out:
            img[2].save_jpg(self.filename + '-' + str(img[0]) + '.jpg')

class Download:
    def __init__(self, name):
        self.name = name
        self.filename = name.replace('/', '')

    def execute(self):
        # get page 1 + page 1 img + links to other pages
        print 'Getting page list'
        page_raw = requests.get(sites['mangareader']['url'] + self.name)
        page = BeautifulSoup(page_raw.text, 'html.parser')
        img1 = sites['mangareader']['img'](page)

        jpg_raw = requests.get(img1)
        jpg = Image.open(StringIO(jpg_raw.content))
        jpg.save_jpg(self.filename + '-0.jpg')

        # TODO save into a cbz

        # get list of pages
        pages = sites['mangareader']['page_list'](page)
        pages = [(i+1,p) for i, p in enumerate(pages[1:])]
        # pprint(pages)

        # get multiple pages
        start_time = time()
        q = Gevent_download(pages, workers=10, filename=self.filename)
        res = q.execute()
        # res.sort()
        # pprint(res)
        q.save()
        print '>>> Time:', time()-start_time


if __name__ == '__main__':
    d = Download('/naruto/5')
    d.execute()
