// app/auth/page.tsx
'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation';

//outside random words
const words = ["PAPER", "DOOR", "THIN", "GRASS", "GRAY", "MINE", "CHALK", "CAT", "DOG", "RUN", "FAST", "BIG", "RED", "SUN", "HAT", "CUP", "PEN", "BOX", "CAR", "SKY", "SIT", "MAP", "NET", "BED", "TOY", "PIG", "PAN"];
const PORT = process.env.NEXT_PUBLIC_PORT || '2026';

const surrealWords = () => {
  const dex = Math.floor(Math.random() * words.length);
  return words[dex];
};

export default function AuthPage() {

  const router = useRouter();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [succcesId, setSuccessId] = useState<number | null>(null);
  const [loginMessage, setLoginMessage] = useState('');

  const [selectedWords, setSelectedWords] = useState<string[]>(["","","","","",""]);

  useEffect(() => {
    let timeoutId: NodeJS.Timeout;
    const pickMe = () => {
      const shuffle = [...words].sort(() => 0.5 - Math.random());
      setSelectedWords(shuffle.slice(0,6));

      const delay = Math.random() * 1000 + 500;
      timeoutId = setTimeout(pickMe, delay);
    };

    pickMe();
    return () => clearTimeout(timeoutId);
  },[]);

  useEffect(() => {
    if (succcesId || error || loginMessage) {
      const timer = setTimeout(() => {
        setSuccessId(null);
        setError('');
        setLoginMessage('');
      }, 2000);
      return () => clearTimeout(timer);
    }
  }, [succcesId, error, loginMessage]);

  const handleSignup = async (e: React.SyntheticEvent) => {
    e.preventDefault();

    setSuccessId(null);
    setLoginMessage('');
    setError('');
    
    if (!email || !password || /\s/.test(email)){
      setError('PLEASE FILL (no space) or COMPLETE');
      return;
    }

    if(!email.endsWith("@gmail.com")){
      setError("Email must ends with '@gmail.com'");
        return;
    }

    setLoading(true);

    try {
      const res = await fetch(`http://localhost:${PORT}/api/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      if (!res.ok) {
        const errorMessage = await res.text();
        setError(errorMessage)
        return;
      }

      const data = await res.json();

      setSuccessId(data.id);

    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  const handleLogin = async (e: React.SyntheticEvent) => {
    e.preventDefault();

    setSuccessId(null);
    setLoginMessage('');
    setLoading(true);
    setError('');

    if (!email || !password){
      setError('PLEASE FILL or COMPLETE')
      setLoading(false);
      return;
    }

    try {
      const res = await fetch(`http://localhost:${PORT}/api/login`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({email,password})
      });

      if (!res.ok){
        const errorMessage = await res.text();
        setError(errorMessage)
        return;
      }

      const data = await res.json();

      localStorage.setItem('user_token_swing', data.token);
      localStorage.setItem('username_swing', data.username);

      setLoginMessage('Login Successful');
      
      router.push('/dashboard');
    }catch(err: any){
      setError(err.message)
    }finally{
      setLoading(false);
    }
  }

  const isVisible = loading || succcesId !== null || error !== '' || loginMessage !== '';

  return (
    <main className="min-h-screen bg-black text-white font-mono p-8 flex flex-col justify-center items-center relative">
      <div className="absolute top-20 w-full max-w-md px-4 flex flex-col items-center pointer-events-none">
        <div className={`w-full p-3 bg-zinc-950 border ${error ? 'border-red-900 text-red-400' : 'border-zinc-700 text-white'} text-xs text-center shadow-2xl transition-all duration-700 ease-out ${isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 -translate-y-2'}`}>
          
          {succcesId && !loading && `[ REGISTERED SUCCESSFULLY! ID: ${succcesId} ]`}
          {loginMessage && !loading && `[${loginMessage}]`}
          {error && !loading && `[ ${error} ]`}
        </div>
      </div>

      <div className="border border-zinc-800 p-6 w-full max-w-md bg-zinc-950">
        <h1 className="text-sm tracking-widest text-zinc-400 mb-6 uppercase">
          {selectedWords[0]}_{selectedWords[1]} {selectedWords[2]}_{selectedWords[3]} {selectedWords[4]}_{selectedWords[5]}
        </h1>
        <div className="flex flex-col gap-4">
          <input 
            type="email" 
            placeholder="EMAIL"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="bg-black border border-zinc-700 p-2 text-sm focus:outline-none focus:border-white"
          />
          <input 
            type="password" 
            placeholder="PASSWORD"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="bg-black border border-zinc-700 p-2 text-sm focus:outline-none focus:border-white"
          />

          <div className="flex gap-2 mt-4">
            <button 
              type="button"
              onClick={handleLogin}
              className="flex-1 bg-white text-black text-xs font-bold py-2 hover:bg-zinc-200"
            >
              LOGIN
            </button>
            <button 
              type="button"
              onClick={handleSignup}
              className="flex-1 border border-zinc-700 text-xs py-2 hover:border-white"
            >
              SIGN_UP
            </button>
          </div>

          <button 
            type="button"
            onClick={() => {
              localStorage.removeItem('user_token_swing');
              localStorage.removeItem('username_swing');
              router.push('/dashboard');
            }}
            className="mt-2 text-zinc-500 text-xs hover:text-white underline text-center"
          >
            Enter Viewer Mode
          </button>
        </div>
      </div>
    </main>
  )
}